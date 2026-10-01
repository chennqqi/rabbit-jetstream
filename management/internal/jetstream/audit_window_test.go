package jetstream

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestAuditWindowFiltersAndEmptyContinuation(t *testing.T) {
	backend := newFakeJS()
	client := &Client{js: backend, metadataReplicas: 1}
	filter := AuditFilter{Resource: "orders", Actor: "owner", Phase: "outcome", Action: "queue.apply", Outcome: "accepted", RequestID: "target"}
	for i := 1; i <= 300; i++ {
		event := AuditEvent{ID: fmt.Sprint(i), RequestID: "other", ResourceName: "other", Actor: "other", Phase: "intent", Action: "queue.apply", Outcome: "accepted"}
		if i == 1 {
			event.RequestID = "target"
			event.ResourceName = "orders"
			event.Actor = "owner"
			event.Phase = "outcome"
		}
		if _, err := client.RecordAudit(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	first, err := client.AuditWindow(context.Background(), filter, nil)
	if err != nil || len(first.Items) != 0 || first.NextBefore == nil || first.Scanned != 256 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	// A new arrival cannot move the exclusive continuation back to a seen row.
	if _, err = client.RecordAudit(context.Background(), AuditEvent{ID: "new", RequestID: "target", ResourceName: "orders", Actor: "owner", Phase: "outcome", Action: "queue.apply", Outcome: "accepted"}); err != nil {
		t.Fatal(err)
	}
	next, err := client.AuditWindow(context.Background(), filter, first.NextBefore)
	if err != nil || len(next.Items) != 1 || next.Items[0].Sequence != 1 || next.NextBefore != nil {
		t.Fatalf("next=%+v err=%v", next, err)
	}
	all, err := client.AuditWindow(context.Background(), AuditFilter{}, nil)
	if err != nil || len(all.Items) != 256 || all.Items[0].Sequence != 301 {
		t.Fatalf("all=%+v err=%v", all, err)
	}
	wrong := filter
	wrong.Actor = "Owner"
	page, err := client.AuditWindow(context.Background(), wrong, first.NextBefore)
	if err != nil || len(page.Items) != 0 {
		t.Fatal("filter was not case-sensitive")
	}
	backend.streams[auditStream].messages[301].Data = []byte("broken")
	page, err = client.AuditWindow(context.Background(), AuditFilter{}, nil)
	if err == nil || len(page.Items) != 0 {
		t.Fatal("corruption returned partial evidence")
	}
}

func TestAuditWindowAbsentDoesNotCreateStorage(t *testing.T) {
	backend := newFakeJS()
	client := &Client{js: backend}
	page, err := client.AuditWindow(context.Background(), AuditFilter{}, nil)
	if err != nil || page.StreamPresent || page.Items == nil || len(backend.streams) != 0 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}

func TestAuditWindowExactTimeRange(t *testing.T) {
	backend := newFakeJS()
	client := &Client{js: backend, metadataReplicas: 1}
	base := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		if _, err := client.RecordAudit(context.Background(), AuditEvent{ID: fmt.Sprint(i), RequestID: "r", ResourceName: "q", Action: "queue.apply", Time: base.Add(time.Duration(i))}); err != nil {
			t.Fatal(err)
		}
	}
	filter := AuditFilter{From: "2026-09-10T08:00:00.000000001+08:00", Until: "2026-09-10T00:00:00.000000002Z"}
	page, err := client.AuditWindow(context.Background(), filter, nil)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != "1" {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	for _, value := range []string{"2026-02-30T00:00:00Z", "2026-09-10T24:00:00Z", "2026-09-10T00:00:00", "2026-09-10T00:00:00.1234567890Z", "2026-09-10T00:00:00+24:00"} {
		if _, _, err := (AuditFilter{From: value}).TimeBounds(); err == nil {
			t.Fatalf("invalid timestamp accepted: %s", value)
		}
	}
	if _, _, err := (AuditFilter{From: filter.Until, Until: filter.Until}).TimeBounds(); err == nil {
		t.Fatal("empty range accepted")
	}
}
