package jetstream

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	jsapi "github.com/nats-io/nats.go/jetstream"
)

func TestAuditPersistsProtectedStreamAndListsNewestFirst(t *testing.T) {
	backend := newFakeJS()
	client := &Client{js: backend, metadataReplicas: 3}
	for _, id := range []string{"one", "two", "three"} {
		sequence, err := client.RecordAudit(context.Background(), AuditEvent{ID: id, RequestID: "request", Time: time.Now(), Phase: "intent", Action: "queue.apply", ResourceKind: "Queue", ResourceName: "orders", Actor: "actor", Outcome: "attempted"})
		if err != nil {
			t.Fatal(err)
		}
		if sequence == 0 {
			t.Fatal("audit sequence was not returned")
		}
	}
	stream := backend.streams[auditStream]
	if stream == nil || stream.info.Config.Replicas != 3 || !stream.info.Config.DenyDelete || !stream.info.Config.DenyPurge || stream.info.Config.MaxAge != 365*24*time.Hour || stream.info.Config.MaxBytes != 1<<30 || stream.info.Config.Compression != jsapi.S2Compression {
		t.Fatalf("audit Stream config=%#v", stream)
	}
	page, err := client.ListAudit(context.Background(), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || len(page.Items) != 2 || page.Items[0].ID != "two" || page.Items[1].ID != "one" || page.Items[0].Sequence != 2 {
		t.Fatalf("page=%#v", page)
	}
}

func TestAuditFailurePaths(t *testing.T) {
	client := &Client{js: newFakeJS(), metadataReplicas: 1}
	if _, err := client.RecordAudit(context.Background(), AuditEvent{}); err == nil {
		t.Fatal("invalid event succeeded")
	}
	page, err := client.ListAudit(context.Background(), 0, 10)
	if err != nil || page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("page=%#v err=%v", page, err)
	}

	backend := newFakeJS()
	backend.createStreamErr = errors.New("storage unavailable")
	client = &Client{js: backend, metadataReplicas: 1}
	event := AuditEvent{ID: "id", RequestID: "request", Action: "queue.delete", ResourceName: "orders"}
	if _, err := client.RecordAudit(context.Background(), event); err == nil || !strings.Contains(err.Error(), "ensure audit Stream") {
		t.Fatalf("err=%v", err)
	}
	backend.createStreamErr = nil
	backend.publishErr = errors.New("publish unavailable")
	if _, err := client.RecordAudit(context.Background(), event); err == nil || !strings.Contains(err.Error(), "persist audit") {
		t.Fatalf("err=%v", err)
	}
}

func TestAuditRejectsCorruptStoredEvent(t *testing.T) {
	backend := newFakeJS()
	client := &Client{js: backend, metadataReplicas: 1}
	event := AuditEvent{ID: "id", RequestID: "request", Action: "queue.apply", ResourceName: "orders"}
	if _, err := client.RecordAudit(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	backend.streams[auditStream].messages[1].Data = []byte("not-json")
	if _, err := client.ListAudit(context.Background(), 0, 1); err == nil || !strings.Contains(err.Error(), "decode audit event") {
		t.Fatalf("err=%v", err)
	}
}
