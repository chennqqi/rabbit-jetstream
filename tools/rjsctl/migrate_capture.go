package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/chennqqi/rabbit-jetstream/internal/migration"
	"github.com/chennqqi/rabbit-jetstream/internal/redact"
)

type evidenceWriter struct{ file *os.File }

func newEvidenceWriter(path string) (*evidenceWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &evidenceWriter{f}, nil
}
func (w *evidenceWriter) write(id string, body []byte) error {
	d := sha256.Sum256(body)
	if err := json.NewEncoder(w.file).Encode(migration.Observation{ID: id, SHA256: hex.EncodeToString(d[:]), Size: int64(len(body))}); err != nil {
		return err
	}
	return w.file.Sync()
}
func (w *evidenceWriter) close() error { return w.file.Close() }

func runCapture(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: rjsctl migrate capture rabbitmq|jetstream [flags]")
	}
	switch args[0] {
	case "rabbitmq":
		return captureRabbitMQ(args[1:], stdout, stderr)
	case "jetstream":
		return captureJetStream(args[1:], stdout, stderr)
	default:
		return errors.New("usage: rjsctl migrate capture rabbitmq|jetstream [flags]")
	}
}

func captureRabbitMQ(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("migrate capture rabbitmq", flag.ContinueOnError)
	fs.SetOutput(stderr)
	url := fs.String("url", os.Getenv("RJS_RABBITMQ_URL"), "RabbitMQ URL")
	queue := fs.String("queue", "", "dedicated shadow queue")
	output := fs.String("output", "", "new evidence file")
	count := fs.Int("count", 0, "messages to capture")
	timeout := fs.Duration("timeout", 5*time.Minute, "capture deadline")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *url == "" || *queue == "" || *output == "" || *count < 1 || *timeout <= 0 {
		return errors.New("usage: rjsctl migrate capture rabbitmq --queue QUEUE --output FILE --count N [--url URL] [--timeout DURATION]")
	}
	conn, err := amqp.Dial(*url)
	if err != nil {
		return sanitizedConnectionError("RabbitMQ", *url, err, os.Getenv("RJS_RABBITMQ_URL"))
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	w, err := newEvidenceWriter(*output)
	if err != nil {
		return fmt.Errorf("create evidence: %w", err)
	}
	defer w.close()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	deliveries, err := ch.ConsumeWithContext(ctx, *queue, "", false, true, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume shadow queue: %w", err)
	}
	for i := 0; i < *count; i++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("capture incomplete: wrote %d/%d: %w", i, *count, ctx.Err())
		case msg, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("capture incomplete: delivery channel closed after %d/%d", i, *count)
			}
			id := msg.MessageId
			if id == "" {
				_ = msg.Nack(false, true)
				return errors.New("RabbitMQ shadow message lacks message_id")
			}
			if err := w.write(id, msg.Body); err != nil {
				_ = msg.Nack(false, true)
				return err
			}
			if err := msg.Ack(false); err != nil {
				return err
			}
		}
	}
	fmt.Fprintf(stdout, "captured %d RabbitMQ observations to %s\n", *count, *output)
	return nil
}

func captureJetStream(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("migrate capture jetstream", flag.ContinueOnError)
	fs.SetOutput(stderr)
	url := fs.String("url", envOr("RJS_NATS_URL", "nats://127.0.0.1:4222"), "NATS URL")
	stream := fs.String("stream", "", "stream name")
	filter := fs.String("filter", "", "filter subject")
	output := fs.String("output", "", "new evidence file")
	count := fs.Int("count", 0, "messages to capture")
	timeout := fs.Duration("timeout", 5*time.Minute, "capture deadline")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *stream == "" || *filter == "" || *output == "" || *count < 1 || *timeout <= 0 {
		return errors.New("usage: rjsctl migrate capture jetstream --stream STREAM --filter SUBJECT --output FILE --count N [flags]")
	}
	opts := []nats.Option{nats.Timeout(5 * time.Second)}
	if u := os.Getenv("RJS_NATS_USER"); u != "" {
		opts = append(opts, nats.UserInfo(u, os.Getenv("RJS_NATS_PASSWORD")))
	}
	if creds := os.Getenv("RJS_NATS_CREDS"); creds != "" {
		opts = append(opts, nats.UserCredentials(creds))
	}
	nc, err := nats.Connect(*url, opts...)
	if err != nil {
		return sanitizedConnectionError("NATS", *url, err, os.Getenv("RJS_NATS_PASSWORD"))
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	info, err := js.StreamInfo(*stream)
	if err != nil {
		return fmt.Errorf("inspect shadow Stream: %w", err)
	}
	if err := validateShadowRetention(info.Config.Retention); err != nil {
		return err
	}
	w, err := newEvidenceWriter(*output)
	if err != nil {
		return fmt.Errorf("create evidence: %w", err)
	}
	defer w.close()
	sub, err := js.PullSubscribe(*filter, "", nats.BindStream(*stream), nats.DeliverNew(), nats.ManualAck())
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	deadline := time.Now().Add(*timeout)
	for i := 0; i < *count; i++ {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("capture incomplete: wrote %d/%d", i, *count)
		}
		msgs, err := sub.Fetch(1, nats.MaxWait(remaining))
		if err != nil {
			return fmt.Errorf("capture incomplete after %d/%d: %w", i, *count, err)
		}
		msg := msgs[0]
		id := msg.Header.Get("Nats-Msg-Id")
		if id == "" {
			_ = msg.Nak()
			return errors.New("JetStream shadow message lacks Nats-Msg-Id")
		}
		if err := w.write(id, msg.Data); err != nil {
			_ = msg.Nak()
			return err
		}
		if err := msg.AckSync(); err != nil {
			return err
		}
	}
	fmt.Fprintf(stdout, "captured %d JetStream observations to %s\n", *count, *output)
	return nil
}

func validateShadowRetention(policy nats.RetentionPolicy) error {
	if policy == nats.WorkQueuePolicy {
		return errors.New("refusing to attach a shadow consumer to a WorkQueue-retention Stream; use a dedicated Limits-retention shadow Stream")
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func sanitizedConnectionError(system, endpoint string, err error, secrets ...string) error {
	message := err.Error()
	if parsed, parseErr := url.Parse(endpoint); parseErr == nil && parsed.User != nil {
		secrets = append(secrets, parsed.User.Username())
		if password, ok := parsed.User.Password(); ok {
			secrets = append(secrets, password)
		}
	}
	for _, secret := range append(secrets, endpoint) {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[redacted]")
		}
	}
	return fmt.Errorf("connect %s %s: %s", system, redact.URL(endpoint), message)
}
