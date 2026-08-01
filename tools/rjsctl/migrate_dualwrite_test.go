package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/migration"
	"github.com/nats-io/nats.go"
	amqp "github.com/rabbitmq/amqp091-go"
)

type fakeDualPublisher struct {
	ids []string
	err error
}

func (p *fakeDualPublisher) publish(record migration.DualWriteRecord) error {
	p.ids = append(p.ids, record.ID)
	return p.err
}

type fakeConfirmationJournal struct {
	events []string
	err    error
}
type fakeJetStreamPublisher struct {
	msg *nats.Msg
	err error
}
type fakeRabbitConfirmation struct {
	ack bool
	err error
}

func (c fakeRabbitConfirmation) WaitContext(context.Context) (bool, error) { return c.ack, c.err }

type fakeRabbitChannel struct {
	message      amqp.Publishing
	confirmation fakeRabbitConfirmation
	err          error
}

func (c *fakeRabbitChannel) publish(_ context.Context, _, _ string, message amqp.Publishing) (rabbitConfirmation, error) {
	c.message = message
	return c.confirmation, c.err
}

func (p *fakeJetStreamPublisher) PublishMsg(msg *nats.Msg, _ ...nats.PubOpt) (*nats.PubAck, error) {
	p.msg = msg
	return &nats.PubAck{Stream: "SHADOW", Sequence: 1}, p.err
}

func (j *fakeConfirmationJournal) confirm(id, _, broker string) error {
	j.events = append(j.events, id+":"+broker)
	return j.err
}

func TestNATSDualPublisherPreservesMessageAndForcesStableID(t *testing.T) {
	fake := &fakeJetStreamPublisher{}
	publisher := natsDualPublisher{fake, "shadow.events", time.Second}
	record := migration.DualWriteRecord{ID: "one", Payload: []byte("payload"), Headers: map[string]string{"tenant": "one", "Nats-Msg-Id": "wrong"}}
	if err := publisher.publish(record); err != nil {
		t.Fatal(err)
	}
	if fake.msg.Subject != "shadow.events" || string(fake.msg.Data) != "payload" || fake.msg.Header.Get("tenant") != "one" || fake.msg.Header.Get("Nats-Msg-Id") != "one" {
		t.Fatalf("msg=%+v", fake.msg)
	}
	fake.err = errors.New("publish failed")
	if err := publisher.publish(record); err == nil {
		t.Fatal("publish error ignored")
	}
}

func TestRabbitDualPublisherRequiresAckAndRejectsReturn(t *testing.T) {
	record := migration.DualWriteRecord{ID: "one", Payload: []byte("payload"), ContentType: "text/plain", Headers: map[string]string{"tenant": "one"}}
	channel := &fakeRabbitChannel{confirmation: fakeRabbitConfirmation{ack: true}}
	returns := make(chan amqp.Return, 1)
	publisher := rabbitDualPublisher{channel, returns, "exchange", "key", time.Second}
	if err := publisher.publish(record); err != nil {
		t.Fatal(err)
	}
	if channel.message.MessageId != "one" || channel.message.ContentType != "text/plain" || channel.message.Headers["tenant"] != "one" {
		t.Fatalf("message=%+v", channel.message)
	}
	channel.confirmation.ack = false
	if err := publisher.publish(record); err == nil {
		t.Fatal("negative ack accepted")
	}
	channel.confirmation.ack = true
	channel.confirmation.err = errors.New("confirm timeout")
	if err := publisher.publish(record); err == nil {
		t.Fatal("confirm error ignored")
	}
	channel.confirmation.err = nil
	returns <- amqp.Return{ReplyText: "NO_ROUTE"}
	if err := publisher.publish(record); err == nil {
		t.Fatal("unroutable publish accepted")
	}
	channel.err = errors.New("publish")
	if err := publisher.publish(record); err == nil {
		t.Fatal("publish error ignored")
	}
}

func TestDualWriteJournalIsDurableLockedAndResumable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.ndjson")
	j, states, unlock, err := openDualWriteJournal(path)
	if err != nil || len(states) != 0 {
		t.Fatalf("states=%v err=%v", states, err)
	}
	if _, _, _, err := openDualWriteJournal(path); err == nil {
		t.Fatal("concurrent journal writer accepted")
	}
	if err := j.confirm("one", strings.Repeat("a", 64), "rabbitmq"); err != nil {
		t.Fatal(err)
	}
	if err := j.close(); err != nil {
		t.Fatal(err)
	}
	unlock()
	j, states, unlock, err = openDualWriteJournal(path)
	if err != nil || !states["one"].RabbitConfirmed {
		t.Fatalf("states=%v err=%v", states, err)
	}
	_ = j.close()
	unlock()
}

func TestDualWriteJournalRejectsCorruptionAndCleansLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.ndjson")
	if err := os.WriteFile(path, []byte("not-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := openDualWriteJournal(path); err == nil {
		t.Fatal("corrupt journal accepted")
	}
	if _, err := os.Stat(path + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("stale lock: %v", err)
	}
	directory := t.TempDir()
	if _, _, _, err := openDualWriteJournal(directory); err == nil {
		t.Fatal("directory accepted as journal")
	}
	if _, err := os.Stat(directory + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("stale lock: %v", err)
	}
}

func TestExecuteDualWriteResumesOnlyMissingSides(t *testing.T) {
	records := []migration.DualWriteRecord{{ID: "one", Payload: []byte("one")}, {ID: "two", Payload: []byte("two")}}
	states := map[string]migration.DualWriteState{"one": {SHA256: migration.DualWriteDigest(records[0]), RabbitConfirmed: true}}
	rabbit, natsPublisher := &fakeDualPublisher{}, &fakeDualPublisher{}
	journal := &fakeConfirmationJournal{}
	completed, err := executeDualWrite(records, states, journal, rabbit, natsPublisher)
	if err != nil || completed != 2 {
		t.Fatalf("completed=%d err=%v", completed, err)
	}
	if strings.Join(rabbit.ids, ",") != "two" || strings.Join(natsPublisher.ids, ",") != "one,two" || strings.Join(journal.events, ",") != "one:jetstream,two:rabbitmq,two:jetstream" {
		t.Fatalf("rabbit=%v nats=%v events=%v", rabbit.ids, natsPublisher.ids, journal.events)
	}
}

func TestExecuteDualWriteSkipsFullyConfirmedAndStopsOnNATSJournalFailure(t *testing.T) {
	record := migration.DualWriteRecord{ID: "one", Payload: []byte("one")}
	digest := migration.DualWriteDigest(record)
	rabbit, natsPublisher := &fakeDualPublisher{}, &fakeDualPublisher{}
	journal := &fakeConfirmationJournal{}
	completed, err := executeDualWrite([]migration.DualWriteRecord{record}, map[string]migration.DualWriteState{"one": {SHA256: digest, RabbitConfirmed: true, NATSConfirmed: true}}, journal, rabbit, natsPublisher)
	if err != nil || completed != 1 || len(rabbit.ids) != 0 || len(natsPublisher.ids) != 0 {
		t.Fatalf("completed=%d err=%v rabbit=%v nats=%v", completed, err, rabbit.ids, natsPublisher.ids)
	}
	journal.err = errors.New("disk")
	completed, err = executeDualWrite([]migration.DualWriteRecord{record}, map[string]migration.DualWriteState{"one": {SHA256: digest, RabbitConfirmed: true}}, journal, rabbit, natsPublisher)
	if err == nil || completed != 0 {
		t.Fatalf("completed=%d err=%v", completed, err)
	}
}

func TestExecuteDualWriteStopsWithoutRecordingFailedConfirm(t *testing.T) {
	records := []migration.DualWriteRecord{{ID: "one", Payload: []byte("one")}}
	for _, test := range []struct{ rabbitErr, natsErr, journalErr error }{{rabbitErr: errors.New("rabbit")}, {natsErr: errors.New("nats")}, {journalErr: errors.New("disk")}} {
		rabbit := &fakeDualPublisher{err: test.rabbitErr}
		natsPublisher := &fakeDualPublisher{err: test.natsErr}
		journal := &fakeConfirmationJournal{err: test.journalErr}
		completed, err := executeDualWrite(records, map[string]migration.DualWriteState{}, journal, rabbit, natsPublisher)
		if err == nil || completed != 0 {
			t.Fatalf("completed=%d err=%v", completed, err)
		}
	}
}

func TestDualWriteRejectsUsageAndInvalidInputBeforeConnecting(t *testing.T) {
	if err := runDualWrite(nil, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("empty usage accepted")
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "input.ndjson")
	if err := os.WriteFile(input, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := runDualWrite([]string{"--input", input, "--journal", filepath.Join(dir, "journal"), "--rabbit-url", "amqp://localhost", "--exchange", "x", "--routing-key", "k", "--subject", "s"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("invalid input accepted")
	}
}

func TestDualWriteConnectionFailureLeavesRecoverableEmptyJournal(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.ndjson")
	journal := filepath.Join(dir, "journal.ndjson")
	if err := os.WriteFile(input, []byte(`{"id":"one","payload_base64":"b25l"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := runMigrate([]string{"dual-write", "--input", input, "--journal", journal, "--rabbit-url", "amqp://user:secret@127.0.0.1:1/", "--exchange", "x", "--routing-key", "k", "--subject", "s", "--timeout", "1s"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("err=%v", err)
	}
	if _, err := os.Stat(journal + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("stale lock: %v", err)
	}
	data, err := os.ReadFile(journal)
	if err != nil || len(data) != 0 {
		t.Fatalf("journal=%q err=%v", data, err)
	}
}
