package jetstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	jsapi "github.com/nats-io/nats.go/jetstream"
)

const (
	auditStream  = "RJS_AUDIT_EVENTS"
	auditSubject = "rjs.audit.events"
)

type AuditEvent struct {
	ID           string    `json:"id"`
	RequestID    string    `json:"requestId"`
	IntentID     string    `json:"intentId,omitempty"`
	Time         time.Time `json:"time"`
	Phase        string    `json:"phase"`
	Action       string    `json:"action"`
	ResourceKind string    `json:"resourceKind"`
	ResourceName string    `json:"resourceName"`
	Actor        string    `json:"actor"`
	ActorRole    string    `json:"actorRole"`
	Tenant       string    `json:"tenant,omitempty"`
	SourceIP     string    `json:"sourceIp,omitempty"`
	Outcome      string    `json:"outcome"`
	HTTPStatus   int       `json:"httpStatus,omitempty"`
	ErrorCode    string    `json:"errorCode,omitempty"`
	Revision     string    `json:"revision,omitempty"`
	Force        bool      `json:"force,omitempty"`
	Sequence     uint64    `json:"sequence,omitempty"`
}

type AuditPage struct {
	Items  []AuditEvent `json:"items"`
	Total  int          `json:"total"`
	Offset int          `json:"offset"`
	Limit  int          `json:"limit"`
}

func (c *Client) RecordAudit(ctx context.Context, event AuditEvent) (uint64, error) {
	if event.ID == "" || event.RequestID == "" || event.Action == "" || event.ResourceName == "" {
		return 0, errors.New("audit event is missing required identity fields")
	}
	if err := c.ensureAuditStream(ctx); err != nil {
		return 0, err
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return 0, fmt.Errorf("encode audit event: %w", err)
	}
	message := nats.NewMsg(auditSubject)
	message.Header.Set("Content-Type", "application/json")
	message.Header.Set("Nats-Msg-Id", event.ID)
	message.Data = encoded
	ack, err := c.js.PublishMsg(ctx, message)
	if err != nil {
		return 0, fmt.Errorf("persist audit event: %w", err)
	}
	return ack.Sequence, nil
}

func (c *Client) ListAudit(ctx context.Context, offset, limit int) (AuditPage, error) {
	if offset < 0 || limit < 1 || limit > 200 {
		return AuditPage{}, errors.New("invalid audit pagination")
	}
	page := AuditPage{Offset: offset, Limit: limit, Items: []AuditEvent{}}
	stream, err := c.js.Stream(ctx, auditStream)
	if errors.Is(err, jsapi.ErrStreamNotFound) {
		return page, nil
	}
	if err != nil {
		return page, fmt.Errorf("open audit Stream: %w", err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		return page, fmt.Errorf("read audit Stream state: %w", err)
	}
	page.Total = int(info.State.Msgs)
	if limit < 1 || offset >= page.Total {
		return page, nil
	}
	// Offset counts retained messages, not sequence positions. Expiry or gaps
	// must not make subsequent pages repeat records from an earlier page.
	sequence := info.State.LastSeq
	skipped := 0
	for sequence > 0 && sequence >= info.State.FirstSeq && len(page.Items) < limit {
		if err := ctx.Err(); err != nil {
			return AuditPage{}, err
		}
		message, getErr := stream.GetMsg(ctx, sequence)
		if getErr == nil {
			if skipped < offset {
				skipped++
				sequence--
				continue
			}
			var event AuditEvent
			if decodeErr := json.Unmarshal(message.Data, &event); decodeErr != nil {
				return AuditPage{}, fmt.Errorf("decode audit event at sequence %d: %w", sequence, decodeErr)
			}
			if event.ID == "" || event.RequestID == "" {
				return AuditPage{}, fmt.Errorf("audit event identity missing at sequence %d", sequence)
			}
			event.Sequence = message.Sequence
			page.Items = append(page.Items, event)
		} else if !errors.Is(getErr, jsapi.ErrMsgNotFound) {
			return AuditPage{}, fmt.Errorf("read audit event at sequence %d: %w", sequence, getErr)
		}
		if sequence == 0 {
			break
		}
		sequence--
	}
	return page, nil
}

func (c *Client) ensureAuditStream(ctx context.Context) error {
	c.auditMu.Lock()
	defer c.auditMu.Unlock()
	if c.auditReady {
		return nil
	}
	_, err := c.js.CreateOrUpdateStream(ctx, jsapi.StreamConfig{
		Name: auditStream, Description: "Durable management mutation audit events",
		Subjects: []string{auditSubject}, Retention: jsapi.LimitsPolicy,
		Storage: jsapi.FileStorage, Replicas: c.metadataReplicas,
		MaxAge: 365 * 24 * time.Hour, MaxBytes: 1 << 30, MaxMsgSize: 64 << 10,
		Discard: jsapi.DiscardOld, DenyDelete: true, DenyPurge: true,
		Compression: jsapi.S2Compression,
		Metadata:    map[string]string{"rabbit-jetstream.io/component": "audit"},
	})
	if err != nil {
		return fmt.Errorf("ensure audit Stream: %w", err)
	}
	c.auditReady = true
	return nil
}
