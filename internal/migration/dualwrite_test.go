package migration

import (
	"strings"
	"testing"
	"time"
)

func TestDualWriteRecordsAndJournalResume(t *testing.T) {
	records, err := ReadDualWriteRecords(strings.NewReader(`{"id":"one","payload_base64":"cGF5bG9hZA==","content_type":"text/plain","headers":{"tenant":"one"}}`))
	if err != nil || len(records) != 1 {
		t.Fatalf("records=%+v err=%v", records, err)
	}
	digest := DualWriteDigest(records[0])
	journal := `{"id":"one","sha256":"` + digest + `","broker":"rabbitmq","confirmed_at":"` + time.Now().UTC().Format(time.RFC3339Nano) + `"}`
	states, err := ReadDualWriteJournal(strings.NewReader(journal))
	if err != nil {
		t.Fatal(err)
	}
	if !states["one"].RabbitConfirmed || states["one"].NATSConfirmed {
		t.Fatalf("state=%+v", states["one"])
	}
	if err := ValidateDualWriteResume(records, states); err != nil {
		t.Fatal(err)
	}
	records[0].Headers["tenant"] = "changed"
	if err := ValidateDualWriteResume(records, states); err == nil {
		t.Fatal("content drift was accepted")
	}
}

func TestDualWriteEvidenceRejectsAmbiguousInput(t *testing.T) {
	invalidInputs := []string{"", `{}`, `{"id":"one","payload_base64":"***"}`, `{"id":"one","payload_base64":"YQ=="} {}`, `{"id":"one","payload_base64":"YQ=="}` + "\n" + `{"id":"one","payload_base64":"YQ=="}`}
	for _, input := range invalidInputs {
		if _, err := ReadDualWriteRecords(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted input %q", input)
		}
	}
	invalidJournals := []string{`{}`, `{"id":"one","sha256":"bad","broker":"rabbitmq","confirmed_at":"2026-01-01T00:00:00Z"}`, `{"id":"one","sha256":"` + strings.Repeat("a", 64) + `","broker":"other","confirmed_at":"2026-01-01T00:00:00Z"} {}`}
	for _, input := range invalidJournals {
		if _, err := ReadDualWriteJournal(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted journal %q", input)
		}
	}
}

func TestDualWriteJournalCombinesBothBrokerConfirmations(t *testing.T) {
	digest := strings.Repeat("a", 64)
	when := time.Now().UTC().Format(time.RFC3339Nano)
	journal := `{"id":"one","sha256":"` + digest + `","broker":"rabbitmq","confirmed_at":"` + when + `"}` + "\n" + `{"id":"one","sha256":"` + digest + `","broker":"jetstream","confirmed_at":"` + when + `"}`
	states, err := ReadDualWriteJournal(strings.NewReader(journal))
	if err != nil || !states["one"].RabbitConfirmed || !states["one"].NATSConfirmed {
		t.Fatalf("states=%+v err=%v", states, err)
	}
	conflict := journal + "\n" + `{"id":"one","sha256":"` + strings.Repeat("b", 64) + `","broker":"rabbitmq","confirmed_at":"` + when + `"}`
	if _, err := ReadDualWriteJournal(strings.NewReader(conflict)); err == nil {
		t.Fatal("journal digest conflict accepted")
	}
}
