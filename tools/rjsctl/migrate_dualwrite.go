package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/migration"
	"github.com/chennqqi/rabbit-jetstream/internal/natsclient"
	"github.com/nats-io/nats.go"
	amqp "github.com/rabbitmq/amqp091-go"
)

type dualWriteJournal struct {
	file    *os.File
	encoder *json.Encoder
}

func openDualWriteJournal(path string) (*dualWriteJournal, map[string]migration.DualWriteState, func(), error) {
	lockPath := path + ".lock"
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("acquire journal lock: %w", err)
	}
	_, _ = fmt.Fprintf(lock, "pid=%d started=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))
	_ = lock.Close()
	unlock := func() { _ = os.Remove(lockPath) }
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		unlock()
		return nil, nil, nil, err
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		unlock()
		return nil, nil, nil, err
	}
	states, err := migration.ReadDualWriteJournal(bufio.NewReader(file))
	if err != nil {
		file.Close()
		unlock()
		return nil, nil, nil, err
	}
	if _, err = file.Seek(0, io.SeekEnd); err != nil {
		file.Close()
		unlock()
		return nil, nil, nil, err
	}
	return &dualWriteJournal{file, json.NewEncoder(file)}, states, unlock, nil
}
func (j *dualWriteJournal) confirm(id, digest, broker string) error {
	if err := j.encoder.Encode(migration.DualWriteEvent{ID: id, SHA256: digest, Broker: broker, ConfirmedAt: time.Now().UTC()}); err != nil {
		return err
	}
	return j.file.Sync()
}
func (j *dualWriteJournal) close() error { return j.file.Close() }

type confirmationJournal interface {
	confirm(string, string, string) error
}
type dualPublisher interface {
	publish(migration.DualWriteRecord) error
}
type rabbitDualPublisher struct {
	channel       rabbitPublishChannel
	returns       <-chan amqp.Return
	exchange, key string
	timeout       time.Duration
}
type rabbitConfirmation interface {
	WaitContext(context.Context) (bool, error)
}
type rabbitPublishChannel interface {
	publish(context.Context, string, string, amqp.Publishing) (rabbitConfirmation, error)
}
type amqpPublishChannel struct{ channel *amqp.Channel }

func (c amqpPublishChannel) publish(ctx context.Context, exchange, key string, message amqp.Publishing) (rabbitConfirmation, error) {
	return c.channel.PublishWithDeferredConfirmWithContext(ctx, exchange, key, true, false, message)
}

type jetStreamPublisher interface {
	PublishMsg(*nats.Msg, ...nats.PubOpt) (*nats.PubAck, error)
}
type natsDualPublisher struct {
	js      jetStreamPublisher
	subject string
	timeout time.Duration
}

func (p rabbitDualPublisher) publish(record migration.DualWriteRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	headers := amqp.Table{}
	for k, v := range record.Headers {
		headers[k] = v
	}
	confirmation, err := p.channel.publish(ctx, p.exchange, p.key, amqp.Publishing{DeliveryMode: amqp.Persistent, MessageId: record.ID, ContentType: record.ContentType, Headers: headers, Body: record.Payload})
	if err != nil {
		return err
	}
	ack, err := confirmation.WaitContext(ctx)
	if err != nil {
		return err
	}
	if !ack {
		return errors.New("RabbitMQ publish was negatively acknowledged")
	}
	select {
	case returned := <-p.returns:
		return fmt.Errorf("unroutable: %s", returned.ReplyText)
	default:
		return nil
	}
}
func (p natsDualPublisher) publish(record migration.DualWriteRecord) error {
	msg := nats.NewMsg(p.subject)
	for k, v := range record.Headers {
		msg.Header.Set(k, v)
	}
	msg.Header.Set("Nats-Msg-Id", record.ID)
	msg.Data = record.Payload
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	_, err := p.js.PublishMsg(msg, nats.Context(ctx))
	return err
}
func executeDualWrite(records []migration.DualWriteRecord, states map[string]migration.DualWriteState, journal confirmationJournal, rabbit, jetstream dualPublisher) (int, error) {
	completed := 0
	for _, record := range records {
		state := states[record.ID]
		digest := migration.DualWriteDigest(record)
		if !state.RabbitConfirmed {
			if err := rabbit.publish(record); err != nil {
				return completed, fmt.Errorf("publish %q to RabbitMQ: %w", record.ID, err)
			}
			if err := journal.confirm(record.ID, digest, "rabbitmq"); err != nil {
				return completed, err
			}
			state.RabbitConfirmed = true
		}
		if !state.NATSConfirmed {
			if err := jetstream.publish(record); err != nil {
				return completed, fmt.Errorf("publish %q to JetStream: %w", record.ID, err)
			}
			if err := journal.confirm(record.ID, digest, "jetstream"); err != nil {
				return completed, err
			}
			state.NATSConfirmed = true
		}
		states[record.ID] = state
		if state.RabbitConfirmed && state.NATSConfirmed {
			completed++
		}
	}
	return completed, nil
}

func runDualWrite(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("migrate dual-write", flag.ContinueOnError)
	fs.SetOutput(stderr)
	inputPath := fs.String("input", "", "NDJSON outbox input")
	journalPath := fs.String("journal", "", "append-only confirmation journal")
	rabbitURL := fs.String("rabbit-url", os.Getenv("RJS_RABBITMQ_URL"), "RabbitMQ URL")
	exchange := fs.String("exchange", "", "RabbitMQ exchange")
	routingKey := fs.String("routing-key", "", "RabbitMQ routing key")
	natsURL := fs.String("nats-url", envOr("RJS_NATS_URL", "nats://127.0.0.1:4222"), "NATS URL")
	subject := fs.String("subject", "", "JetStream subject")
	timeout := fs.Duration("timeout", 10*time.Second, "per-publish confirm timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *inputPath == "" || *journalPath == "" || *rabbitURL == "" || *exchange == "" || *routingKey == "" || *subject == "" || *timeout <= 0 {
		return errors.New("usage: rjsctl migrate dual-write --input FILE --journal FILE --exchange EXCHANGE --routing-key KEY --subject SUBJECT [connection flags]")
	}
	input, err := os.Open(*inputPath)
	if err != nil {
		return err
	}
	records, err := migration.ReadDualWriteRecords(input)
	_ = input.Close()
	if err != nil {
		return err
	}
	journal, states, unlock, err := openDualWriteJournal(*journalPath)
	if err != nil {
		return err
	}
	defer unlock()
	defer journal.close()
	if err := migration.ValidateDualWriteResume(records, states); err != nil {
		return err
	}
	rabbit, err := amqp.Dial(*rabbitURL)
	if err != nil {
		return sanitizedConnectionError("RabbitMQ", *rabbitURL, err)
	}
	defer rabbit.Close()
	channel, err := rabbit.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()
	if err := channel.Confirm(false); err != nil {
		return err
	}
	returns := channel.NotifyReturn(make(chan amqp.Return, 1))
	opts, err := natsclient.Options(natsclient.FromEnv())
	if err != nil {
		return err
	}
	opts = append(opts, nats.Timeout(*timeout))
	nc, err := nats.Connect(*natsURL, opts...)
	if err != nil {
		return sanitizedConnectionError("NATS", *natsURL, err, os.Getenv("RJS_NATS_PASSWORD"))
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	completed, err := executeDualWrite(records, states, journal, rabbitDualPublisher{amqpPublishChannel{channel}, returns, *exchange, *routingKey, *timeout}, natsDualPublisher{js, *subject, *timeout})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "dual-write confirmed %d/%d messages; journal=%s\n", completed, len(records), *journalPath)
	return nil
}
