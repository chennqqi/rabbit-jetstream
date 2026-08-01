package jetstream

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/nats-io/nats.go"
	jsapi "github.com/nats-io/nats.go/jetstream"
)

type fakeJS struct {
	jsapi.JetStream
	mu                                                                      sync.Mutex
	kv                                                                      *memoryKV
	streams                                                                 map[string]*fakeStream
	account                                                                 *jsapi.AccountInfo
	accountErr                                                              error
	worker                                                                  jsapi.Consumer
	published                                                               []*nats.Msg
	publishErr                                                              error
	kvErr, createKVErr, createStreamErr, createConsumerErr, deleteStreamErr error
}

func newFakeJS() *fakeJS {
	return &fakeJS{streams: make(map[string]*fakeStream), account: &jsapi.AccountInfo{API: jsapi.APIStats{Level: 1}}}
}

func (f *fakeJS) AccountInfo(context.Context) (*jsapi.AccountInfo, error) {
	return f.account, f.accountErr
}
func (f *fakeJS) KeyValue(context.Context, string) (jsapi.KeyValue, error) {
	if f.kvErr != nil {
		return nil, f.kvErr
	}
	if f.kv == nil {
		return nil, jsapi.ErrBucketNotFound
	}
	return f.kv, nil
}
func (f *fakeJS) CreateKeyValue(_ context.Context, _ jsapi.KeyValueConfig) (jsapi.KeyValue, error) {
	if f.createKVErr != nil {
		return nil, f.createKVErr
	}
	if f.kv != nil {
		return nil, jsapi.ErrBucketExists
	}
	f.kv = newMemoryKV()
	return f.kv, nil
}
func (f *fakeJS) CreateOrUpdateStream(_ context.Context, cfg jsapi.StreamConfig) (jsapi.Stream, error) {
	if f.createStreamErr != nil {
		return nil, f.createStreamErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	stream := f.streams[cfg.Name]
	if stream == nil {
		stream = &fakeStream{consumers: make(map[string]*jsapi.ConsumerInfo), messages: make(map[uint64]*jsapi.RawStreamMsg)}
		f.streams[cfg.Name] = stream
	}
	state := streamState(stream)
	stream.info = &jsapi.StreamInfo{Config: cfg, State: state}
	return stream, nil
}
func (f *fakeJS) CreateOrUpdateConsumer(_ context.Context, streamName string, cfg jsapi.ConsumerConfig) (jsapi.Consumer, error) {
	if f.createConsumerErr != nil {
		return nil, f.createConsumerErr
	}
	stream := f.streams[streamName]
	if stream == nil {
		return nil, jsapi.ErrStreamNotFound
	}
	info := &jsapi.ConsumerInfo{Stream: streamName, Name: cfg.Name, Config: cfg}
	stream.consumers[cfg.Name] = info
	return &fakeConsumer{info: info}, nil
}
func (f *fakeJS) Stream(_ context.Context, name string) (jsapi.Stream, error) {
	stream := f.streams[name]
	if stream == nil {
		return nil, jsapi.ErrStreamNotFound
	}
	return stream, nil
}
func (f *fakeJS) DeleteStream(_ context.Context, name string) error {
	if f.deleteStreamErr != nil {
		return f.deleteStreamErr
	}
	if f.streams[name] == nil {
		return jsapi.ErrStreamNotFound
	}
	delete(f.streams, name)
	return nil
}
func (f *fakeJS) Consumer(context.Context, string, string) (jsapi.Consumer, error) {
	if f.worker == nil {
		return nil, jsapi.ErrConsumerNotFound
	}
	return f.worker, nil
}
func (f *fakeJS) PublishMsg(_ context.Context, message *nats.Msg, _ ...jsapi.PublishOpt) (*jsapi.PubAck, error) {
	if f.publishErr != nil {
		return nil, f.publishErr
	}
	f.published = append(f.published, &nats.Msg{Subject: message.Subject, Data: append([]byte(nil), message.Data...), Header: cloneHeader(message.Header)})
	ack := &jsapi.PubAck{}
	for name, stream := range f.streams {
		if stream.info == nil || !slices.Contains(stream.info.Config.Subjects, message.Subject) {
			continue
		}
		sequence := stream.info.State.LastSeq + 1
		if stream.messages == nil {
			stream.messages = make(map[uint64]*jsapi.RawStreamMsg)
		}
		stream.messages[sequence] = &jsapi.RawStreamMsg{Subject: message.Subject, Sequence: sequence, Header: cloneHeader(message.Header), Data: append([]byte(nil), message.Data...), Time: time.Now()}
		stream.info.State = streamState(stream)
		ack.Stream, ack.Sequence = name, sequence
		break
	}
	return ack, nil
}
func (f *fakeJS) ListStreams(context.Context, ...jsapi.StreamListOpt) jsapi.StreamInfoLister {
	items := make([]*jsapi.StreamInfo, 0, len(f.streams))
	for _, stream := range f.streams {
		items = append(items, stream.info)
	}
	return &fakeStreamLister{items: items}
}

type fakeStream struct {
	jsapi.Stream
	info      *jsapi.StreamInfo
	consumers map[string]*jsapi.ConsumerInfo
	messages  map[uint64]*jsapi.RawStreamMsg
}

func (f *fakeStream) CachedInfo() *jsapi.StreamInfo { return f.info }
func (f *fakeStream) Info(context.Context, ...jsapi.StreamInfoOpt) (*jsapi.StreamInfo, error) {
	return f.info, nil
}
func (f *fakeStream) ListConsumers(context.Context) jsapi.ConsumerInfoLister {
	items := make([]*jsapi.ConsumerInfo, 0, len(f.consumers))
	for _, consumer := range f.consumers {
		items = append(items, consumer)
	}
	return &fakeConsumerLister{items: items}
}
func (f *fakeStream) GetMsg(_ context.Context, sequence uint64, _ ...jsapi.GetMsgOpt) (*jsapi.RawStreamMsg, error) {
	message := f.messages[sequence]
	if message == nil {
		return nil, jsapi.ErrMsgNotFound
	}
	return message, nil
}
func (f *fakeStream) DeleteMsg(_ context.Context, sequence uint64) error {
	if f.messages[sequence] == nil {
		return jsapi.ErrMsgNotFound
	}
	delete(f.messages, sequence)
	return nil
}

type fakeConsumer struct {
	jsapi.Consumer
	info  *jsapi.ConsumerInfo
	batch jsapi.MessageBatch
}

func (f *fakeConsumer) CachedInfo() *jsapi.ConsumerInfo             { return f.info }
func (f *fakeConsumer) FetchNoWait(int) (jsapi.MessageBatch, error) { return f.batch, nil }

type fakeBatch struct {
	messages []jsapi.Msg
	err      error
}

func (f *fakeBatch) Messages() <-chan jsapi.Msg {
	ch := make(chan jsapi.Msg, len(f.messages))
	for _, message := range f.messages {
		ch <- message
	}
	close(ch)
	return ch
}
func (f *fakeBatch) Error() error { return f.err }

type fakeMessage struct {
	data                []byte
	acked, nakd, termed bool
}

func (f *fakeMessage) Metadata() (*jsapi.MsgMetadata, error) { return &jsapi.MsgMetadata{}, nil }
func (f *fakeMessage) Data() []byte                          { return f.data }
func (f *fakeMessage) Headers() nats.Header                  { return nil }
func (f *fakeMessage) Subject() string                       { return "advisory" }
func (f *fakeMessage) Reply() string                         { return "" }
func (f *fakeMessage) Ack() error                            { f.acked = true; return nil }
func (f *fakeMessage) DoubleAck(context.Context) error       { f.acked = true; return nil }
func (f *fakeMessage) Nak() error                            { f.nakd = true; return nil }
func (f *fakeMessage) NakWithDelay(time.Duration) error      { f.nakd = true; return nil }
func (f *fakeMessage) InProgress() error                     { return nil }
func (f *fakeMessage) Term() error                           { f.termed = true; return nil }
func (f *fakeMessage) TermWithReason(string) error           { f.termed = true; return nil }

type fakeStreamLister struct {
	items []*jsapi.StreamInfo
	err   error
}

func (f *fakeStreamLister) Info() <-chan *jsapi.StreamInfo {
	ch := make(chan *jsapi.StreamInfo, len(f.items))
	for _, item := range f.items {
		ch <- item
	}
	close(ch)
	return ch
}
func (f *fakeStreamLister) Err() error { return f.err }

type fakeConsumerLister struct {
	items []*jsapi.ConsumerInfo
	err   error
}

func (f *fakeConsumerLister) Info() <-chan *jsapi.ConsumerInfo {
	ch := make(chan *jsapi.ConsumerInfo, len(f.items))
	for _, item := range f.items {
		ch <- item
	}
	close(ch)
	return ch
}
func (f *fakeConsumerLister) Err() error { return f.err }

func streamState(stream *fakeStream) jsapi.StreamState {
	if stream == nil || len(stream.messages) == 0 {
		return jsapi.StreamState{}
	}
	var first, last uint64
	for sequence := range stream.messages {
		if first == 0 || sequence < first {
			first = sequence
		}
		if sequence > last {
			last = sequence
		}
	}
	return jsapi.StreamState{Msgs: uint64(len(stream.messages)), FirstSeq: first, LastSeq: last}
}

type memoryKV struct {
	jsapi.KeyValue
	mu       sync.Mutex
	revision uint64
	values   map[string]memoryEntry
}
type memoryEntry struct {
	value    []byte
	revision uint64
	created  time.Time
}
type fakeKVEntry struct {
	key string
	memoryEntry
}

func newMemoryKV() *memoryKV                       { return &memoryKV{values: make(map[string]memoryEntry)} }
func (e *fakeKVEntry) Bucket() string              { return "META" }
func (e *fakeKVEntry) Key() string                 { return e.key }
func (e *fakeKVEntry) Value() []byte               { return append([]byte(nil), e.value...) }
func (e *fakeKVEntry) Revision() uint64            { return e.revision }
func (e *fakeKVEntry) Created() time.Time          { return e.created }
func (e *fakeKVEntry) Delta() uint64               { return 0 }
func (e *fakeKVEntry) Operation() jsapi.KeyValueOp { return jsapi.KeyValuePut }
func (k *memoryKV) next(value []byte) memoryEntry {
	k.revision++
	return memoryEntry{value: append([]byte(nil), value...), revision: k.revision, created: time.Now()}
}
func (k *memoryKV) Get(_ context.Context, key string) (jsapi.KeyValueEntry, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	value, ok := k.values[key]
	if !ok {
		return nil, jsapi.ErrKeyNotFound
	}
	return &fakeKVEntry{key: key, memoryEntry: value}, nil
}
func (k *memoryKV) Put(_ context.Context, key string, value []byte) (uint64, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	entry := k.next(value)
	k.values[key] = entry
	return entry.revision, nil
}
func (k *memoryKV) Create(_ context.Context, key string, value []byte, _ ...jsapi.KVCreateOpt) (uint64, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, ok := k.values[key]; ok {
		return 0, jsapi.ErrKeyExists
	}
	entry := k.next(value)
	k.values[key] = entry
	return entry.revision, nil
}
func (k *memoryKV) Update(_ context.Context, key string, value []byte, revision uint64) (uint64, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	current, ok := k.values[key]
	if !ok || current.revision != revision {
		return 0, jsapi.ErrKeyExists
	}
	entry := k.next(value)
	k.values[key] = entry
	return entry.revision, nil
}
func (k *memoryKV) Delete(_ context.Context, key string, _ ...jsapi.KVDeleteOpt) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, ok := k.values[key]; !ok {
		return jsapi.ErrKeyNotFound
	}
	delete(k.values, key)
	k.revision++
	return nil
}
func (k *memoryKV) Keys(context.Context, ...jsapi.WatchOpt) ([]string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	keys := make([]string, 0, len(k.values))
	for key := range k.values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return nil, jsapi.ErrNoKeysFound
	}
	return keys, nil
}

func testClient() (*Client, *fakeJS) {
	backend := newFakeJS()
	return &Client{js: backend, metadataBucket: "META", metadataReplicas: 1}, backend
}

func testQueuePlan(t *testing.T, name string) topology.Plan {
	t.Helper()
	document := `apiVersion: rabbit-jetstream.io/v1alpha1
kind: Queue
metadata: {name: ` + name + `}
spec:
  subjects: [` + name + `.messages]
  replicas: 1
`
	queue, err := topology.ParseQueue(strings.NewReader(document))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := topology.BuildPlan(*queue)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestClientAccountStreamsAndConsumers(t *testing.T) {
	client, backend := testClient()
	backend.account = &jsapi.AccountInfo{Tier: jsapi.Tier{Memory: 10, Store: 20, Streams: 2, Consumers: 1}, API: jsapi.APIStats{Total: 4, Errors: 1, Level: 2}}
	_, _ = backend.CreateOrUpdateStream(context.Background(), jsapi.StreamConfig{Name: "zeta", Subjects: []string{"z"}, Storage: jsapi.FileStorage})
	_, _ = backend.CreateOrUpdateStream(context.Background(), jsapi.StreamConfig{Name: "alpha", Subjects: []string{"a"}, Storage: jsapi.MemoryStorage})
	_, _ = backend.CreateOrUpdateConsumer(context.Background(), "alpha", jsapi.ConsumerConfig{Name: "worker", Durable: "worker", AckPolicy: jsapi.AckExplicitPolicy})
	if err := client.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}
	account, err := client.AccountInfo(context.Background())
	if err != nil || account.StorageUsed != 20 || account.APIErrors != 1 {
		t.Fatalf("account=%#v err=%v", account, err)
	}
	streams, err := client.ListStreams(context.Background())
	if err != nil || len(streams) != 2 || streams[0].Name != "alpha" {
		t.Fatalf("streams=%#v err=%v", streams, err)
	}
	stream, err := client.Stream(context.Background(), "alpha")
	if err != nil || stream.Storage != "memory" {
		t.Fatalf("stream=%#v err=%v", stream, err)
	}
	consumers, err := client.ListConsumers(context.Background(), "alpha")
	if err != nil || len(consumers) != 1 || consumers[0].Name != "worker" {
		t.Fatalf("consumers=%#v err=%v", consumers, err)
	}
	if _, err := client.Stream(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestClientControllerLeaseLifecycle(t *testing.T) {
	client, _ := testClient()
	ctx := context.Background()
	leader, err := client.AcquireControllerLease(ctx, "one", time.Minute)
	if err != nil || !leader {
		t.Fatalf("leader=%v err=%v", leader, err)
	}
	leader, err = client.AcquireControllerLease(ctx, "two", time.Minute)
	if err != nil || leader {
		t.Fatalf("leader=%v err=%v", leader, err)
	}
	leader, err = client.AcquireControllerLease(ctx, "one", time.Minute)
	if err != nil || !leader {
		t.Fatalf("renew leader=%v err=%v", leader, err)
	}
	if err := client.ReleaseControllerLease(ctx, "two"); err != nil {
		t.Fatal(err)
	}
	if err := client.ReleaseControllerLease(ctx, "one"); err != nil {
		t.Fatal(err)
	}
	leader, err = client.AcquireControllerLease(ctx, "two", time.Minute)
	if err != nil || !leader {
		t.Fatalf("takeover leader=%v err=%v", leader, err)
	}
}

func TestClientApplyDeclarationAndDeleteLifecycle(t *testing.T) {
	client, backend := testClient()
	ctx := context.Background()
	plan := testQueuePlan(t, "orders")
	result, err := client.Apply(ctx, plan)
	if err != nil || result.Blocked || backend.streams[plan.Stream.Name] == nil {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	declaration, err := client.Declaration(ctx, "orders")
	if err != nil || declaration.Revision != plan.Revision || declaration.KVRevision == 0 {
		t.Fatalf("declaration=%#v err=%v", declaration, err)
	}
	declarations, err := client.ListDeclarations(ctx)
	if err != nil || len(declarations) != 1 {
		t.Fatalf("declarations=%#v err=%v", declarations, err)
	}
	if _, err := client.ApplyConditional(ctx, plan, ApplyPrecondition{CreateOnly: true}); !errors.Is(err, ErrConflict) {
		t.Fatalf("err=%v", err)
	}
	result, err = client.ApplyDeclaration(ctx, *declaration)
	if err != nil || result.Status != "noop" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	backend.streams[plan.Stream.Name].info.State.Msgs = 1
	deleted, err := client.DeleteQueue(ctx, "orders", false)
	if err != nil || !deleted.Blocked || deleted.Messages != 1 {
		t.Fatalf("deleted=%#v err=%v", deleted, err)
	}
	deleted, err = client.DeleteQueue(ctx, "orders", true)
	if err != nil || deleted.Status != "deleted" {
		t.Fatalf("deleted=%#v err=%v", deleted, err)
	}
	if _, err := client.Declaration(ctx, "orders"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestClientListDeclarationsRejectsCorruptRecord(t *testing.T) {
	client, backend := testClient()
	backend.kv = newMemoryKV()
	_, _ = backend.kv.Put(context.Background(), "queues.bad", []byte("{"))
	if _, err := client.ListDeclarations(context.Background()); err == nil || !strings.Contains(err.Error(), "decode declaration") {
		t.Fatalf("err=%v", err)
	}
	record := topology.Declaration{APIVersion: topology.DeclarationAPIVersion, Queue: "ok", Revision: "r"}
	encoded, _ := json.Marshal(record)
	_, _ = backend.kv.Put(context.Background(), "other.key", []byte("ignored"))
	_, _ = backend.kv.Put(context.Background(), "queues.ok", encoded)
}

func TestClientProcessesDeadLetterAdvisories(t *testing.T) {
	client, backend := testClient()
	plan := testQueuePlan(t, "source")
	plan.DeadLetter = &topology.DeadLetterPlan{Queue: "target", Stream: topology.StreamName("target"), Worker: "worker", Mechanism: "advisory-republish"}
	backend.streams[plan.Stream.Name] = &fakeStream{
		info: &jsapi.StreamInfo{Config: jsapi.StreamConfig{Name: plan.Stream.Name}}, consumers: make(map[string]*jsapi.ConsumerInfo),
		messages: map[uint64]*jsapi.RawStreamMsg{7: {Subject: "source.messages", Sequence: 7, Header: nats.Header{"Traceparent": {"trace"}}, Data: []byte("poison")}},
	}
	valid, _ := json.Marshal(MaxDeliverAdvisory{Type: "io.nats.jetstream.advisory.v1.max_deliver", ID: "event", Stream: plan.Stream.Name, Consumer: plan.Consumer.Name, StreamSeq: 7, Deliveries: 3})
	unrelated, _ := json.Marshal(MaxDeliverAdvisory{Stream: "OTHER", Consumer: "OTHER", StreamSeq: 1})
	good, bad, other := &fakeMessage{data: valid}, &fakeMessage{data: []byte("{")}, &fakeMessage{data: unrelated}
	backend.worker = &fakeConsumer{batch: &fakeBatch{messages: []jsapi.Msg{bad, other, good}}}
	result, err := client.ProcessDeadLetters(context.Background(), []topology.Declaration{{Queue: "source", Plan: plan}}, 10)
	if err != nil || result.Processed != 3 || result.Moved != 1 || result.Ignored != 2 || result.Failed != 0 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if !bad.termed || !other.acked || !good.acked || len(backend.published) != 1 {
		t.Fatalf("bad=%#v other=%#v good=%#v published=%d", bad, other, good, len(backend.published))
	}
	published := backend.published[0]
	if published.Subject != topology.QueueIngressSubject("target") || string(published.Data) != "poison" || published.Header.Get("Traceparent") != "trace" || published.Header.Get("Rjs-Dead-Letter-Source-Queue") != "source" {
		t.Fatalf("published=%#v", published)
	}
	if backend.streams[plan.Stream.Name].messages[7] != nil {
		t.Fatal("source message was not deleted")
	}
}

func TestClientRetriesFailedDeadLetterPublish(t *testing.T) {
	client, backend := testClient()
	plan := testQueuePlan(t, "source")
	plan.DeadLetter = &topology.DeadLetterPlan{Queue: "target"}
	backend.streams[plan.Stream.Name] = &fakeStream{
		info: &jsapi.StreamInfo{Config: jsapi.StreamConfig{Name: plan.Stream.Name}}, consumers: make(map[string]*jsapi.ConsumerInfo),
		messages: map[uint64]*jsapi.RawStreamMsg{1: {Subject: "source.messages", Sequence: 1, Data: []byte("poison")}},
	}
	payload, _ := json.Marshal(MaxDeliverAdvisory{Stream: plan.Stream.Name, Consumer: plan.Consumer.Name, StreamSeq: 1})
	event := &fakeMessage{data: payload}
	backend.worker = &fakeConsumer{batch: &fakeBatch{messages: []jsapi.Msg{event}}}
	backend.publishErr = errors.New("publish unavailable")
	result, err := client.ProcessDeadLetters(context.Background(), []topology.Declaration{{Queue: "source", Plan: plan}}, 1)
	if err != nil || result.Failed != 1 || !event.nakd || backend.streams[plan.Stream.Name].messages[1] == nil {
		t.Fatalf("result=%#v event=%#v err=%v", result, event, err)
	}
}

func TestClientApplyReportsDependencyAndBackendFailures(t *testing.T) {
	ctx := context.Background()
	t.Run("missing DLQ dependency", func(t *testing.T) {
		client, _ := testClient()
		plan := testQueuePlan(t, "source")
		plan.DeadLetter = &topology.DeadLetterPlan{Queue: "missing"}
		if _, err := client.Apply(ctx, plan); err == nil || !strings.Contains(err.Error(), "DLQ dependency") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("stream create", func(t *testing.T) {
		client, backend := testClient()
		backend.createStreamErr = errors.New("stream unavailable")
		if _, err := client.Apply(ctx, testQueuePlan(t, "orders")); err == nil || !strings.Contains(err.Error(), "apply stream") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("consumer create", func(t *testing.T) {
		client, backend := testClient()
		backend.createConsumerErr = errors.New("consumer unavailable")
		if _, err := client.Apply(ctx, testQueuePlan(t, "orders")); err == nil || !strings.Contains(err.Error(), "apply consumer") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("metadata bucket", func(t *testing.T) {
		client, backend := testClient()
		backend.createKVErr = errors.New("storage unavailable")
		if _, err := client.Apply(ctx, testQueuePlan(t, "orders")); err == nil || !strings.Contains(err.Error(), "Queue lock") {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestClientConditionalDeleteAndOwnershipProtection(t *testing.T) {
	client, backend := testClient()
	ctx := context.Background()
	plan := testQueuePlan(t, "orders")
	if _, err := client.Apply(ctx, plan); err != nil {
		t.Fatal(err)
	}
	declaration, _ := client.Declaration(ctx, "orders")
	wrong := declaration.KVRevision + 1
	if _, err := client.DeleteQueueConditional(ctx, "orders", true, ApplyPrecondition{ExpectedRevision: &wrong}); !errors.Is(err, ErrConflict) {
		t.Fatalf("err=%v", err)
	}
	stream := backend.streams[plan.Stream.Name]
	stream.info.Config.Metadata["rabbit-jetstream.io/queue"] = "foreign"
	result, err := client.DeleteQueueConditional(ctx, "orders", true, ApplyPrecondition{ExpectedRevision: &declaration.KVRevision})
	if err != nil || !result.Blocked || !strings.Contains(result.Reason, "not owned") {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	stream.info.Config.Metadata["rabbit-jetstream.io/queue"] = "orders"
	result, err = client.DeleteQueueConditional(ctx, "orders", true, ApplyPrecondition{ExpectedRevision: &declaration.KVRevision})
	if err != nil || result.Status != "deleted" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	result, err = client.DeleteQueue(ctx, "orders", false)
	if err != nil || result.Status != "noop" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestClientEmptyMetadataAndExpiredLease(t *testing.T) {
	client, backend := testClient()
	ctx := context.Background()
	declarations, err := client.ListDeclarations(ctx)
	if err != nil || len(declarations) != 0 {
		t.Fatalf("declarations=%v err=%v", declarations, err)
	}
	backend.kv = newMemoryKV()
	encoded, _ := json.Marshal(controllerLease{Holder: "old", ExpiresAt: time.Now().Add(-time.Minute)})
	_, _ = backend.kv.Put(ctx, "controller.leader", encoded)
	leader, err := client.AcquireControllerLease(ctx, "new", time.Minute)
	if err != nil || !leader {
		t.Fatalf("leader=%v err=%v", leader, err)
	}
	if err := client.ReleaseControllerLease(ctx, "new"); err != nil {
		t.Fatal(err)
	}
	if err := client.ReleaseControllerLease(ctx, "new"); err != nil {
		t.Fatal(err)
	}
}
