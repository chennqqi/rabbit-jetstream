package migration

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

const ReconciliationSchema = "rabbit-jetstream.io/message-reconciliation/v1alpha1"

type Observation struct {
	ID     string `json:"id"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type Thresholds struct {
	MinSource     int `json:"min_source"`
	MaxMissing    int `json:"max_missing"`
	MaxUnexpected int `json:"max_unexpected"`
	MaxMismatch   int `json:"max_mismatch"`
	MaxDuplicates int `json:"max_duplicates"`
	MaxDetails    int `json:"-"`
}

type ReconciliationDetail struct {
	Type   string       `json:"type"`
	ID     string       `json:"id"`
	Source *Observation `json:"source,omitempty"`
	Target *Observation `json:"target,omitempty"`
}

type ReconciliationReport struct {
	Schema           string                 `json:"schema"`
	GeneratedAt      time.Time              `json:"generated_at"`
	Passed           bool                   `json:"passed"`
	Thresholds       Thresholds             `json:"thresholds"`
	SourceRecords    int                    `json:"source_records"`
	TargetRecords    int                    `json:"target_records"`
	SourceUnique     int                    `json:"source_unique"`
	TargetUnique     int                    `json:"target_unique"`
	Matched          int                    `json:"matched"`
	Missing          int                    `json:"missing"`
	Unexpected       int                    `json:"unexpected"`
	ContentMismatch  int                    `json:"content_mismatch"`
	SourceDuplicates int                    `json:"source_duplicates"`
	TargetDuplicates int                    `json:"target_duplicates"`
	DetailsTruncated bool                   `json:"details_truncated"`
	Details          []ReconciliationDetail `json:"details"`
}

type observed struct {
	value Observation
	count int
}

func ReconcileMessages(source, target io.Reader, thresholds Thresholds) (ReconciliationReport, error) {
	if thresholds.MinSource < 0 || thresholds.MaxMissing < 0 || thresholds.MaxUnexpected < 0 || thresholds.MaxMismatch < 0 || thresholds.MaxDuplicates < 0 || thresholds.MaxDetails < 0 {
		return ReconciliationReport{}, errors.New("reconciliation thresholds must be non-negative")
	}
	if thresholds.MaxDetails == 0 {
		thresholds.MaxDetails = 1000
	}
	if thresholds.MinSource == 0 {
		thresholds.MinSource = 1
	}
	sourceValues, sourceRecords, err := readObservations(source, "source")
	if err != nil {
		return ReconciliationReport{}, err
	}
	targetValues, targetRecords, err := readObservations(target, "target")
	if err != nil {
		return ReconciliationReport{}, err
	}
	report := ReconciliationReport{Schema: ReconciliationSchema, GeneratedAt: time.Now().UTC(), Thresholds: thresholds, SourceRecords: sourceRecords, TargetRecords: targetRecords, SourceUnique: len(sourceValues), TargetUnique: len(targetValues), Details: []ReconciliationDetail{}}
	for _, id := range sortedObservationIDs(sourceValues) {
		sourceValue := sourceValues[id]
		report.SourceDuplicates += sourceValue.count - 1
		targetValue, exists := targetValues[id]
		if !exists {
			report.Missing++
			report.addDetail(thresholds.MaxDetails, ReconciliationDetail{Type: "missing", ID: id, Source: &sourceValue.value})
			continue
		}
		if sourceValue.value.SHA256 != targetValue.value.SHA256 || sourceValue.value.Size != targetValue.value.Size {
			report.ContentMismatch++
			report.addDetail(thresholds.MaxDetails, ReconciliationDetail{Type: "content_mismatch", ID: id, Source: &sourceValue.value, Target: &targetValue.value})
		} else {
			report.Matched++
		}
	}
	for _, id := range sortedObservationIDs(targetValues) {
		targetValue := targetValues[id]
		report.TargetDuplicates += targetValue.count - 1
		if _, exists := sourceValues[id]; !exists {
			report.Unexpected++
			report.addDetail(thresholds.MaxDetails, ReconciliationDetail{Type: "unexpected", ID: id, Target: &targetValue.value})
		}
	}
	report.Passed = report.SourceUnique >= thresholds.MinSource && report.Missing <= thresholds.MaxMissing && report.Unexpected <= thresholds.MaxUnexpected && report.ContentMismatch <= thresholds.MaxMismatch && report.SourceDuplicates+report.TargetDuplicates <= thresholds.MaxDuplicates
	return report, nil
}

func sortedObservationIDs(values map[string]observed) []string {
	ids := make([]string, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (r *ReconciliationReport) addDetail(limit int, detail ReconciliationDetail) {
	if len(r.Details) >= limit {
		r.DetailsTruncated = true
		return
	}
	r.Details = append(r.Details, detail)
}

func readObservations(input io.Reader, label string) (map[string]observed, int, error) {
	values := make(map[string]observed)
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	records := 0
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		records++
		decoder := json.NewDecoder(strings.NewReader(line))
		decoder.DisallowUnknownFields()
		var value Observation
		if err := decoder.Decode(&value); err != nil {
			return nil, 0, fmt.Errorf("decode %s observation line %d: %w", label, lineNumber, err)
		}
		var trailing any
		if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			return nil, 0, fmt.Errorf("decode %s observation line %d: trailing JSON value", label, lineNumber)
		}
		if err := validateObservation(&value); err != nil {
			return nil, 0, fmt.Errorf("invalid %s observation line %d: %w", label, lineNumber, err)
		}
		previous, exists := values[value.ID]
		if exists && (previous.value.SHA256 != value.SHA256 || previous.value.Size != value.Size) {
			return nil, 0, fmt.Errorf("conflicting %s observations for message %q", label, value.ID)
		}
		if !exists {
			previous.value = value
		}
		previous.count++
		values[value.ID] = previous
	}
	if err := scanner.Err(); err != nil {
		return nil, 0, fmt.Errorf("read %s observations: %w", label, err)
	}
	return values, records, nil
}

func validateObservation(value *Observation) error {
	value.ID = strings.TrimSpace(value.ID)
	value.SHA256 = strings.ToLower(strings.TrimSpace(value.SHA256))
	if value.ID == "" || len(value.ID) > 512 {
		return errors.New("id must contain 1 to 512 characters")
	}
	digest, err := hex.DecodeString(value.SHA256)
	if err != nil || len(digest) != 32 {
		return errors.New("sha256 must be a 64-character hexadecimal digest")
	}
	if value.Size < 0 {
		return errors.New("size must be non-negative")
	}
	return nil
}
