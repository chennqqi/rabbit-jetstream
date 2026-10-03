package jetstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	jsapi "github.com/nats-io/nats.go/jetstream"
)

const AuditRequestScanLimit = 256

// AuditRequestPage is retained evidence, never proof of operation outcome.
// Before is exclusive; nil starts at the current retained high-water mark.
type AuditRequestPage struct {
	RequestID     string       `json:"requestId"`
	Items         []AuditEvent `json:"items"`
	StreamPresent bool         `json:"streamPresent"`
	FirstSequence uint64       `json:"firstSequence"`
	LastSequence  uint64       `json:"lastSequence"`
	Scanned       int          `json:"scanned"`
	Missing       int          `json:"missing"`
	NextBefore    *uint64      `json:"nextBefore"`
}

func (c *Client) AuditRequest(ctx context.Context, requestID string, before *uint64) (AuditRequestPage, error) {
	return c.scanAudit(ctx, requestID, before, func(event AuditEvent) bool { return event.RequestID == requestID })
}

func (c *Client) scanAudit(ctx context.Context, requestID string, before *uint64, match func(AuditEvent) bool) (AuditRequestPage, error) {
	page := AuditRequestPage{RequestID: requestID, Items: []AuditEvent{}}
	stream, err := c.js.Stream(ctx, auditStream)
	if errors.Is(err, jsapi.ErrStreamNotFound) {
		return page, nil
	}
	if err != nil {
		return page, fmt.Errorf("open audit evidence: %w", err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		return page, fmt.Errorf("read audit evidence range: %w", err)
	}
	page.StreamPresent = true
	page.FirstSequence = info.State.FirstSeq
	page.LastSequence = info.State.LastSeq
	if info.State.Msgs == 0 {
		return page, nil
	}
	sequence := info.State.LastSeq
	if before != nil {
		if *before == 0 {
			return page, nil
		}
		if *before <= sequence {
			sequence = *before - 1
		}
	}
	for sequence >= page.FirstSequence && sequence > 0 && page.Scanned < AuditRequestScanLimit {
		if err := ctx.Err(); err != nil {
			return AuditRequestPage{}, err
		}
		page.Scanned++
		message, getErr := stream.GetMsg(ctx, sequence)
		if errors.Is(getErr, jsapi.ErrMsgNotFound) {
			page.Missing++
		} else if getErr != nil {
			return AuditRequestPage{}, fmt.Errorf("read audit evidence at %d: %w", sequence, getErr)
		} else {
			var event AuditEvent
			if err := json.Unmarshal(message.Data, &event); err != nil {
				return AuditRequestPage{}, fmt.Errorf("decode audit evidence at %d: %w", sequence, err)
			}
			if event.ID == "" || event.RequestID == "" {
				return AuditRequestPage{}, fmt.Errorf("audit evidence identity missing at %d", sequence)
			}
			if match(event) {
				event.Sequence = message.Sequence
				page.Items = append(page.Items, event)
			}
		}
		if sequence == page.FirstSequence {
			break
		}
		if page.Scanned == AuditRequestScanLimit {
			cursor := sequence
			page.NextBefore = &cursor
			break
		}
		sequence--
	}
	return page, nil
}
