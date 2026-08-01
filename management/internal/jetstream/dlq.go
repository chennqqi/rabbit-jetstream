package jetstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/nats-io/nats.go"
	jsapi "github.com/nats-io/nats.go/jetstream"
)

const (
	deadLetterEventStream   = "RJS_DLQ_EVENTS"
	deadLetterEventConsumer = "RJS_DLQ_WORKER"
	maxDeliverySubject      = "$JS.EVENT.ADVISORY.CONSUMER.MAX_DELIVERIES.>"
)

type MaxDeliverAdvisory struct {
	Type       string    `json:"type"`
	ID         string    `json:"id"`
	Time       time.Time `json:"timestamp"`
	Stream     string    `json:"stream"`
	Consumer   string    `json:"consumer"`
	StreamSeq  uint64    `json:"stream_seq"`
	Deliveries uint64    `json:"deliveries"`
}

func (c *Client) ensureDeadLetterInfrastructure(ctx context.Context) error {
	_, err := c.js.CreateOrUpdateStream(ctx, jsapi.StreamConfig{
		Name: deadLetterEventStream, Description: "Durable MaxDeliver advisories for rabbit-jetstream DLQ transfer",
		Subjects: []string{maxDeliverySubject}, Storage: jsapi.FileStorage, Replicas: c.metadataReplicas,
		Retention: jsapi.WorkQueuePolicy, MaxAge: 7 * 24 * time.Hour,
	})
	if err != nil {
		return fmt.Errorf("ensure DLQ advisory stream: %w", err)
	}
	_, err = c.js.CreateOrUpdateConsumer(ctx, deadLetterEventStream, jsapi.ConsumerConfig{
		Name: deadLetterEventConsumer, Durable: deadLetterEventConsumer,
		AckPolicy: jsapi.AckExplicitPolicy, AckWait: 30 * time.Second, MaxDeliver: -1,
		DeliverPolicy: jsapi.DeliverAllPolicy, ReplayPolicy: jsapi.ReplayInstantPolicy,
	})
	if err != nil {
		return fmt.Errorf("ensure DLQ advisory consumer: %w", err)
	}
	return nil
}

// ProcessDeadLetters moves a bounded batch of captured MaxDeliver advisories.
// It publishes with a deterministic message ID before deleting the source, so
// a crash can cause a duplicate but cannot silently lose the payload.
func (c *Client) ProcessDeadLetters(ctx context.Context, declarations []topology.Declaration, limit int) (topology.DeadLetterProcessResult, error) {
	var result topology.DeadLetterProcessResult
	if limit < 1 {
		return result, nil
	}
	bySource := make(map[string]topology.Declaration)
	for _, declaration := range declarations {
		if declaration.Plan.DeadLetter != nil {
			key := declaration.Plan.Stream.Name + "\x00" + declaration.Plan.Consumer.Name
			bySource[key] = declaration
		}
	}
	if len(bySource) == 0 {
		return result, nil
	}
	if err := c.ensureDeadLetterInfrastructure(ctx); err != nil {
		return result, err
	}
	consumer, err := c.js.Consumer(ctx, deadLetterEventStream, deadLetterEventConsumer)
	if err != nil {
		return result, fmt.Errorf("open DLQ advisory consumer: %w", err)
	}
	batch, err := consumer.FetchNoWait(limit)
	if err != nil {
		return result, fmt.Errorf("fetch DLQ advisories: %w", err)
	}
	for event := range batch.Messages() {
		result.Processed++
		var advisory MaxDeliverAdvisory
		if err := json.Unmarshal(event.Data(), &advisory); err != nil {
			result.Ignored++
			_ = event.TermWithReason("invalid MaxDeliver advisory")
			continue
		}
		declaration, ok := bySource[advisory.Stream+"\x00"+advisory.Consumer]
		if !ok || declaration.Plan.DeadLetter == nil {
			result.Ignored++
			_ = event.Ack()
			continue
		}
		if err := c.moveDeadLetter(ctx, declaration, advisory); err != nil {
			result.Failed++
			_ = event.NakWithDelay(time.Second)
			continue
		}
		result.Moved++
		if err := event.DoubleAck(ctx); err != nil {
			return result, fmt.Errorf("ack DLQ advisory: %w", err)
		}
	}
	if err := batch.Error(); err != nil && !errors.Is(err, nats.ErrTimeout) {
		return result, fmt.Errorf("read DLQ advisory batch: %w", err)
	}
	return result, nil
}

func (c *Client) moveDeadLetter(ctx context.Context, declaration topology.Declaration, advisory MaxDeliverAdvisory) error {
	source, err := c.js.Stream(ctx, advisory.Stream)
	if err != nil {
		return fmt.Errorf("open source stream %s: %w", advisory.Stream, err)
	}
	raw, err := source.GetMsg(ctx, advisory.StreamSeq)
	if errors.Is(err, jsapi.ErrMsgNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read source message %s/%d: %w", advisory.Stream, advisory.StreamSeq, err)
	}
	target := declaration.Plan.DeadLetter.Queue
	message := &nats.Msg{Subject: topology.QueueIngressSubject(target), Data: append([]byte(nil), raw.Data...), Header: cloneHeader(raw.Header)}
	message.Header.Set("Rjs-Dead-Letter-Source-Queue", declaration.Queue)
	message.Header.Set("Rjs-Dead-Letter-Source-Subject", raw.Subject)
	message.Header.Set("Rjs-Dead-Letter-Source-Stream", advisory.Stream)
	message.Header.Set("Rjs-Dead-Letter-Source-Consumer", advisory.Consumer)
	message.Header.Set("Rjs-Dead-Letter-Deliveries", fmt.Sprint(advisory.Deliveries))
	id := fmt.Sprintf("rjs-dlq:%s:%s:%d", advisory.Stream, advisory.Consumer, advisory.StreamSeq)
	if _, err := c.js.PublishMsg(ctx, message, jsapi.WithMsgID(id)); err != nil {
		return fmt.Errorf("publish message to DLQ %s: %w", target, err)
	}
	if err := source.DeleteMsg(ctx, advisory.StreamSeq); err != nil && !errors.Is(err, jsapi.ErrMsgNotFound) {
		return fmt.Errorf("delete source message %s/%d: %w", advisory.Stream, advisory.StreamSeq, err)
	}
	return nil
}

func cloneHeader(source nats.Header) nats.Header {
	target := make(nats.Header, len(source)+5)
	for key, values := range source {
		target[key] = append([]string(nil), values...)
	}
	return target
}
