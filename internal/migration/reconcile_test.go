package migration

import (
	"strings"
	"testing"
)

const (
	digestA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func TestReconcileMessagesPassesExactMatch(t *testing.T) {
	source := strings.NewReader(`{"id":"two","sha256":"` + digestB + `","size":20}
{"id":"one","sha256":"` + digestA + `","size":10}`)
	target := strings.NewReader(`{"id":"one","sha256":"` + digestA + `","size":10}
{"id":"two","sha256":"` + digestB + `","size":20}`)
	report, err := ReconcileMessages(source, target, Thresholds{})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed || report.Matched != 2 || report.SourceUnique != 2 || report.TargetUnique != 2 || len(report.Details) != 0 {
		t.Fatalf("report=%+v", report)
	}
}

func TestReconcileMessagesClassifiesDriftAndAppliesThresholds(t *testing.T) {
	source := strings.NewReader(`{"id":"duplicate","sha256":"` + digestA + `","size":10}
{"id":"duplicate","sha256":"` + digestA + `","size":10}
{"id":"missing","sha256":"` + digestA + `","size":10}
{"id":"mismatch","sha256":"` + digestA + `","size":10}`)
	target := strings.NewReader(`{"id":"duplicate","sha256":"` + digestA + `","size":10}
{"id":"mismatch","sha256":"` + digestB + `","size":10}
{"id":"unexpected","sha256":"` + digestA + `","size":10}`)
	report, err := ReconcileMessages(source, target, Thresholds{MaxMissing: 1, MaxUnexpected: 1, MaxMismatch: 1, MaxDuplicates: 1, MaxDetails: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed || report.Matched != 1 || report.Missing != 1 || report.Unexpected != 1 || report.ContentMismatch != 1 || report.SourceDuplicates != 1 || !report.DetailsTruncated || len(report.Details) != 2 {
		t.Fatalf("report=%+v", report)
	}
	report, err = ReconcileMessages(strings.NewReader(`{"id":"one","sha256":"`+digestA+`","size":1}`), strings.NewReader(""), Thresholds{})
	if err != nil || report.Passed || report.Missing != 1 {
		t.Fatalf("report=%+v err=%v", report, err)
	}
}

func TestReconcileMessagesRejectsInvalidAndConflictingEvidence(t *testing.T) {
	tests := []string{
		`{"id":"","sha256":"` + digestA + `","size":1}`,
		`{"id":"one","sha256":"short","size":1}`,
		`{"id":"one","sha256":"` + digestA + `","size":-1}`,
		`{"id":"one","sha256":"` + digestA + `","size":1,"extra":true}`,
		`{"id":"one","sha256":"` + digestA + `","size":1} {}`,
		`{"id":"one","sha256":"` + digestA + `","size":1}` + "\n" + `{"id":"one","sha256":"` + digestB + `","size":1}`,
	}
	for _, source := range tests {
		if _, err := ReconcileMessages(strings.NewReader(source), strings.NewReader(""), Thresholds{}); err == nil {
			t.Fatalf("invalid evidence accepted: %s", source)
		}
	}
	if _, err := ReconcileMessages(strings.NewReader(""), strings.NewReader(""), Thresholds{MaxMissing: -1}); err == nil {
		t.Fatal("negative threshold accepted")
	}
}

func TestReconcileMessagesRejectsEmptyWindow(t *testing.T) {
	report, err := ReconcileMessages(strings.NewReader(""), strings.NewReader(""), Thresholds{})
	if err != nil || report.Passed || report.Thresholds.MinSource != 1 {
		t.Fatalf("report=%+v err=%v", report, err)
	}
}
