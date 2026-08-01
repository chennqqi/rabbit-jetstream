package migration

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

type DualWriteRecord struct {
	ID          string            `json:"id"`
	Payload     []byte            `json:"payload_base64"`
	ContentType string            `json:"content_type,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

type DualWriteEvent struct {
	ID          string    `json:"id"`
	SHA256      string    `json:"sha256"`
	Broker      string    `json:"broker"`
	ConfirmedAt time.Time `json:"confirmed_at"`
}

type DualWriteState struct {
	SHA256          string
	RabbitConfirmed bool
	NATSConfirmed   bool
}

func ReadDualWriteRecords(input io.Reader) ([]DualWriteRecord, error) {
	values := []DualWriteRecord{}
	seen := map[string]string{}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for line := 1; scanner.Scan(); line++ {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		var value DualWriteRecord
		decoder := json.NewDecoder(strings.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("decode dual-write input line %d: %w", line, err)
		}
		if err := requireJSONEOF(decoder); err != nil {
			return nil, fmt.Errorf("decode dual-write input line %d: %w", line, err)
		}
		if value.ID == "" || len(value.ID) > 512 {
			return nil, fmt.Errorf("dual-write input line %d: id must contain 1 to 512 characters", line)
		}
		if value.Payload == nil {
			return nil, fmt.Errorf("dual-write input line %d: payload_base64 is required", line)
		}
		digest := DualWriteDigest(value)
		if previous, exists := seen[value.ID]; exists {
			return nil, fmt.Errorf("duplicate dual-write message id %q (digest %s, previous %s)", value.ID, digest, previous)
		}
		seen[value.ID] = digest
		values = append(values, value)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read dual-write input: %w", err)
	}
	if len(values) == 0 {
		return nil, errors.New("dual-write input contains no messages")
	}
	return values, nil
}

func ReadDualWriteJournal(input io.Reader) (map[string]DualWriteState, error) {
	states := map[string]DualWriteState{}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for line := 1; scanner.Scan(); line++ {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		var event DualWriteEvent
		decoder := json.NewDecoder(strings.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&event); err != nil {
			return nil, fmt.Errorf("decode dual-write journal line %d: %w", line, err)
		}
		if err := requireJSONEOF(decoder); err != nil {
			return nil, fmt.Errorf("decode dual-write journal line %d: %w", line, err)
		}
		digest, digestErr := hex.DecodeString(event.SHA256)
		if event.ID == "" || event.ConfirmedAt.IsZero() || (event.Broker != "rabbitmq" && event.Broker != "jetstream") || digestErr != nil || len(digest) != 32 {
			return nil, fmt.Errorf("invalid dual-write journal line %d", line)
		}
		state := states[event.ID]
		if state.SHA256 != "" && state.SHA256 != event.SHA256 {
			return nil, fmt.Errorf("journal digest conflict for message %q", event.ID)
		}
		state.SHA256 = event.SHA256
		if event.Broker == "rabbitmq" {
			state.RabbitConfirmed = true
		} else {
			state.NATSConfirmed = true
		}
		states[event.ID] = state
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read dual-write journal: %w", err)
	}
	return states, nil
}

func DualWriteDigest(value DualWriteRecord) string {
	canonical, _ := json.Marshal(struct {
		Payload     []byte            `json:"payload"`
		ContentType string            `json:"content_type"`
		Headers     map[string]string `json:"headers"`
	}{value.Payload, value.ContentType, value.Headers})
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:])
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON value")
	}
	return nil
}

func ValidateDualWriteResume(records []DualWriteRecord, states map[string]DualWriteState) error {
	for _, record := range records {
		if state, exists := states[record.ID]; exists && state.SHA256 != DualWriteDigest(record) {
			return fmt.Errorf("journal content mismatch for message %q", record.ID)
		}
	}
	return nil
}
