package jetstream

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

func TestAuditOffsetCountsRecordsNotSequencePositions(t *testing.T) {
	backend := newFakeJS()
	client := &Client{js: backend, metadataReplicas: 1}
	for i := 1; i <= 8; i++ {
		if _, err := client.RecordAudit(context.Background(), AuditEvent{ID: fmt.Sprint(i), RequestID: "request", Action: "queue.apply", ResourceName: "q"}); err != nil {
			t.Fatal(err)
		}
	}
	delete(backend.streams[auditStream].messages, 7)
	delete(backend.streams[auditStream].messages, 4)
	backend.streams[auditStream].info.State = streamState(backend.streams[auditStream])
	var sequences []uint64
	for offset := 0; offset < 6; offset += 2 {
		page, err := client.ListAudit(context.Background(), offset, 2)
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 6 || page.Offset != offset || len(page.Items) != 2 {
			t.Fatalf("page=%+v", page)
		}
		for _, event := range page.Items {
			sequences = append(sequences, event.Sequence)
		}
	}
	if !reflect.DeepEqual(sequences, []uint64{8, 6, 5, 3, 2, 1}) {
		t.Fatalf("repeated or skipped retained events: %v", sequences)
	}
	page, err := client.ListAudit(context.Background(), 99, 2)
	if err != nil || len(page.Items) != 0 || page.Total != 6 {
		t.Fatalf("beyond end: %+v %v", page, err)
	}
}

func TestAuditListFailureReturnsNoPartialEvidence(t *testing.T) {
	backend := newFakeJS()
	client := &Client{js: backend, metadataReplicas: 1}
	for i := 1; i <= 2; i++ {
		if _, err := client.RecordAudit(context.Background(), AuditEvent{ID: fmt.Sprint(i), RequestID: "r", Action: "queue.apply", ResourceName: "q"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, data := range []string{"broken", `{}`} {
		backend.streams[auditStream].messages[1].Data = []byte(data)
		page, err := client.ListAudit(context.Background(), 0, 2)
		if err == nil || len(page.Items) != 0 {
			t.Fatalf("partial evidence returned: %+v %v", page, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.ListAudit(ctx, 0, 2); err == nil {
		t.Fatal("canceled enumeration succeeded")
	}
	for _, args := range [][2]int{{-1, 1}, {0, 0}, {0, 201}} {
		if _, err := client.ListAudit(context.Background(), args[0], args[1]); err == nil {
			t.Fatalf("invalid pagination accepted: %v", args)
		}
	}
}
