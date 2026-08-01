package jetstream

import (
	"encoding/json"
	"testing"

	"github.com/nats-io/nats.go"
)

func TestMaxDeliverAdvisoryContract(t *testing.T) {
	input := []byte(`{"type":"io.nats.jetstream.advisory.v1.max_deliver","id":"event-1","timestamp":"2026-08-01T00:00:00Z","stream":"RJSQ_orders","consumer":"RJSQC_orders","stream_seq":42,"deliveries":5}`)
	var event MaxDeliverAdvisory
	if err := json.Unmarshal(input, &event); err != nil {
		t.Fatal(err)
	}
	if event.Stream != "RJSQ_orders" || event.Consumer != "RJSQC_orders" || event.StreamSeq != 42 || event.Deliveries != 5 || event.ID != "event-1" {
		t.Fatalf("event=%#v", event)
	}
}

func TestCloneHeaderIsIndependent(t *testing.T) {
	source := nats.Header{"Traceparent": {"trace-1"}}
	target := cloneHeader(source)
	target.Set("Traceparent", "trace-2")
	target.Set("Rjs-Dead-Letter-Source-Queue", "orders")
	if source.Get("Traceparent") != "trace-1" || source.Get("Rjs-Dead-Letter-Source-Queue") != "" {
		t.Fatalf("source header was mutated: %#v", source)
	}
}
