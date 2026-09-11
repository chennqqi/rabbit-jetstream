package jetstream

import (
	"context"
	"fmt"
	"testing"
)

func TestAuditRequestBoundedCursorAndRetentionGaps(t *testing.T) {
	backend := newFakeJS()
	client := &Client{js: backend, metadataReplicas: 1}
	for i := 0; i < 300; i++ {
		requestID := "other"
		if i == 0 || i == 299 {
			requestID = "target"
		}
		_, err := client.RecordAudit(context.Background(), AuditEvent{ID: fmt.Sprint(i), RequestID: requestID, Action: "queue.apply", ResourceName: "orders"})
		if err != nil {
			t.Fatal(err)
		}
	}
	delete(backend.streams[auditStream].messages, 299)
	page, err := client.AuditRequest(context.Background(), "target", nil)
	if err != nil || page.Scanned != 256 || page.Missing != 1 || len(page.Items) != 1 || page.Items[0].Sequence != 300 || page.NextBefore == nil {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	next, err := client.AuditRequest(context.Background(), "target", page.NextBefore)
	if err != nil || next.Scanned != 44 || next.NextBefore != nil || len(next.Items) != 1 || next.Items[0].Sequence != 1 {
		t.Fatalf("page=%+v err=%v", next, err)
	}
	zero := uint64(0)
	empty, err := client.AuditRequest(context.Background(), "target", &zero)
	if err != nil || empty.Scanned != 0 {
		t.Fatalf("page=%+v err=%v", empty, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.AuditRequest(ctx, "target", nil); err == nil {
		t.Fatal("canceled scan succeeded")
	}
	backend.streams[auditStream].messages[300].Data = []byte("broken")
	if _, err := client.AuditRequest(context.Background(), "target", nil); err == nil {
		t.Fatal("corruption accepted")
	}
}

func TestAuditRequestAbsentStreamDoesNotCreateStorage(t *testing.T) {
	backend := newFakeJS()
	client := &Client{js: backend}
	page, err := client.AuditRequest(context.Background(), "missing", nil)
	if err != nil || page.StreamPresent || page.Items == nil || len(backend.streams) != 0 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}
