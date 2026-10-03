package prometheus

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A nil *Client reaches these methods when the unconfigured history backend is
// stored as a typed nil inside an interface field. The regression is the
// process-killing panic at endpoint := *c.base.
func TestNilClientMethodsReturnNotConfigured(t *testing.T) {
	var client *Client
	snapshot, err := client.AlertRules(context.Background(), time.Now())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("AlertRules err=%v, want ErrNotConfigured", err)
	}
	if snapshot.Source != "" {
		t.Fatalf("AlertRules snapshot=%+v, want zero value", snapshot)
	}
	rangeResult, err := client.QueryRange(context.Background(), MetricQueueMessages, "orders", Window15Minutes, time.Now())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("QueryRange err=%v, want ErrNotConfigured", err)
	}
	if rangeResult.Schema != "" {
		t.Fatalf("QueryRange result=%+v, want zero value", rangeResult)
	}
}
