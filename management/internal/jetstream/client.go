package jetstream

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/chennqqi/rabbit-jetstream/management/internal/config"
	"github.com/nats-io/nats.go"
	jsapi "github.com/nats-io/nats.go/jetstream"
)

var ErrNotFound = errors.New("resource not found")

type Client struct {
	conn *nats.Conn
	js   jsapi.JetStream
}

func Connect(cfg config.Config) (*Client, error) {
	opts := []nats.Option{
		nats.Name(cfg.Name),
		nats.Timeout(cfg.ConnectTimeout),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
	}
	if cfg.NATSCreds != "" {
		opts = append(opts, nats.UserCredentials(cfg.NATSCreds))
	} else if cfg.NATSUser != "" {
		opts = append(opts, nats.UserInfo(cfg.NATSUser, cfg.NATSPassword))
	}
	conn, err := nats.Connect(strings.Join(strings.Split(cfg.NATSURL, ","), ","), opts...)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}
	js, err := jsapi.New(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create JetStream client: %w", err)
	}
	return &Client{conn: conn, js: js}, nil
}

func (c *Client) Ready(ctx context.Context) error {
	_, err := c.js.AccountInfo(ctx)
	return err
}

func (c *Client) AccountInfo(ctx context.Context) (*Account, error) {
	info, err := c.js.AccountInfo(ctx)
	if err != nil {
		return nil, err
	}
	return &Account{
		Domain: info.Domain, MemoryUsed: info.Memory, StorageUsed: info.Store,
		ReservedMemory: info.ReservedMemory, ReservedStore: info.ReservedStore,
		Streams: info.Streams, Consumers: info.Consumers, APITotal: info.API.Total,
		APIErrors: info.API.Errors, APIInflight: info.API.Inflight, APILevel: info.API.Level,
	}, nil
}

func (c *Client) ListStreams(ctx context.Context) ([]Stream, error) {
	lister := c.js.ListStreams(ctx)
	streams := make([]Stream, 0)
	for info := range lister.Info() {
		streams = append(streams, streamFromInfo(info))
	}
	if err := lister.Err(); err != nil {
		return nil, fmt.Errorf("list streams: %w", err)
	}
	sort.Slice(streams, func(i, j int) bool { return streams[i].Name < streams[j].Name })
	return streams, nil
}

func (c *Client) Stream(ctx context.Context, name string) (*Stream, error) {
	stream, err := c.js.Stream(ctx, name)
	if errors.Is(err, jsapi.ErrStreamNotFound) {
		return nil, fmt.Errorf("%w: stream %s", ErrNotFound, name)
	}
	if err != nil {
		return nil, fmt.Errorf("get stream %s: %w", name, err)
	}
	return ptr(streamFromInfo(stream.CachedInfo())), nil
}

func (c *Client) ListConsumers(ctx context.Context, streamName string) ([]Consumer, error) {
	stream, err := c.js.Stream(ctx, streamName)
	if errors.Is(err, jsapi.ErrStreamNotFound) {
		return nil, fmt.Errorf("%w: stream %s", ErrNotFound, streamName)
	}
	if err != nil {
		return nil, fmt.Errorf("get stream %s: %w", streamName, err)
	}
	lister := stream.ListConsumers(ctx)
	consumers := make([]Consumer, 0)
	for info := range lister.Info() {
		consumers = append(consumers, consumerFromInfo(info))
	}
	if err := lister.Err(); err != nil {
		return nil, fmt.Errorf("list consumers for stream %s: %w", streamName, err)
	}
	sort.Slice(consumers, func(i, j int) bool { return consumers[i].Name < consumers[j].Name })
	return consumers, nil
}

// Apply creates or safely updates the resources in plan. Blocked plans never write.
func (c *Client) Apply(ctx context.Context, plan topology.Plan) (topology.ReconcileResult, error) {
	observed, err := c.observedTopology(ctx, plan)
	if err != nil {
		return topology.ReconcileResult{}, err
	}
	result := topology.Reconcile(plan, observed)
	if result.Blocked || result.Status == "noop" {
		return result, nil
	}
	streamConfig := jsapi.StreamConfig{
		Name: plan.Stream.Name, Subjects: plan.Stream.Subjects, Storage: storageType(plan.Stream.Storage),
		Replicas: plan.Stream.Replicas, Retention: jsapi.WorkQueuePolicy, Discard: jsapi.DiscardOld,
		MaxAge: time.Duration(plan.Stream.MaxAgeNanos), MaxBytes: plan.Stream.MaxBytes,
		MaxMsgs: plan.Stream.MaxMessages, Metadata: cloneMetadata(plan.Stream.Metadata),
	}
	if _, err := c.js.CreateOrUpdateStream(ctx, streamConfig); err != nil {
		return result, fmt.Errorf("apply stream %s: %w", plan.Stream.Name, err)
	}
	consumerConfig := jsapi.ConsumerConfig{
		Name: plan.Consumer.Name, Durable: plan.Consumer.Name,
		FilterSubjects: plan.Consumer.FilterSubjects, DeliverPolicy: jsapi.DeliverAllPolicy,
		AckPolicy: jsapi.AckExplicitPolicy, AckWait: time.Duration(plan.Consumer.AckWaitNanos),
		MaxDeliver: plan.Consumer.MaxDeliver, ReplayPolicy: jsapi.ReplayInstantPolicy,
		Metadata: cloneMetadata(plan.Consumer.Metadata),
	}
	if _, err := c.js.CreateOrUpdateConsumer(ctx, plan.Stream.Name, consumerConfig); err != nil {
		return result, fmt.Errorf("apply consumer %s: %w", plan.Consumer.Name, err)
	}
	return result, nil
}

func (c *Client) DeleteQueue(ctx context.Context, name string, force bool) (topology.DeleteResult, error) {
	result := topology.DeleteResult{Queue: name, Stream: topology.StreamName(name), Forced: force}
	stream, err := c.Stream(ctx, result.Stream)
	if errors.Is(err, ErrNotFound) {
		result.Status = "noop"
		return result, nil
	}
	if err != nil {
		return result, err
	}
	result.Messages = stream.Messages
	if stream.Metadata["rabbit-jetstream.io/queue"] != name {
		result.Status, result.Blocked, result.Reason = "blocked", true, "Stream is not owned by this Queue"
		return result, nil
	}
	if stream.Messages > 0 && !force {
		result.Status, result.Blocked, result.Reason = "blocked", true, "Stream contains messages; force is required"
		return result, nil
	}
	if err := c.js.DeleteStream(ctx, result.Stream); err != nil {
		if errors.Is(err, jsapi.ErrStreamNotFound) {
			result.Status = "noop"
			return result, nil
		}
		return result, fmt.Errorf("delete stream %s: %w", result.Stream, err)
	}
	result.Status = "deleted"
	return result, nil
}

func (c *Client) observedTopology(ctx context.Context, plan topology.Plan) (topology.ObservedTopology, error) {
	var observed topology.ObservedTopology
	stream, err := c.Stream(ctx, plan.Stream.Name)
	if errors.Is(err, ErrNotFound) {
		return observed, nil
	}
	if err != nil {
		return observed, err
	}
	observed.Stream = &topology.ObservedStream{Name: stream.Name, Subjects: stream.Subjects, Storage: stream.Storage, Replicas: stream.Replicas, Retention: stream.Retention, Discard: stream.Discard, MaxAgeNanos: stream.MaxAgeNanos, MaxBytes: stream.MaxBytes, MaxMessages: stream.MaxMessages, Metadata: stream.Metadata}
	consumers, err := c.ListConsumers(ctx, plan.Stream.Name)
	if err != nil {
		return observed, err
	}
	for _, consumer := range consumers {
		if consumer.Name == plan.Consumer.Name {
			observed.Consumer = &topology.ObservedConsumer{Stream: consumer.Stream, Name: consumer.Name, Durable: consumer.Durable, FilterSubject: consumer.FilterSubject, FilterSubjects: consumer.FilterSubjects, Mode: consumer.Mode, DeliverPolicy: consumer.DeliverPolicy, AckPolicy: consumer.AckPolicy, AckWaitNanos: consumer.AckWaitNanos, MaxDeliver: consumer.MaxDeliver, ReplayPolicy: consumer.ReplayPolicy, Metadata: consumer.Metadata}
			break
		}
	}
	return observed, nil
}

func storageType(value string) jsapi.StorageType {
	if value == "memory" {
		return jsapi.MemoryStorage
	}
	return jsapi.FileStorage
}

func streamFromInfo(info *jsapi.StreamInfo) Stream {
	return Stream{
		Name: info.Config.Name, Subjects: info.Config.Subjects, Storage: storageLabel(info.Config.Storage),
		Replicas: info.Config.Replicas, Retention: retentionLabel(info.Config.Retention), Discard: discardLabel(info.Config.Discard),
		MaxAgeNanos: info.Config.MaxAge.Nanoseconds(), MaxBytes: info.Config.MaxBytes, MaxMessages: info.Config.MaxMsgs,
		Metadata: cloneMetadata(info.Config.Metadata), Created: info.Created, Messages: info.State.Msgs,
		Bytes: info.State.Bytes, Consumers: info.State.Consumers, FirstSeq: info.State.FirstSeq,
		LastSeq: info.State.LastSeq, Cluster: clusterFromInfo(info.Cluster),
	}
}

func consumerFromInfo(info *jsapi.ConsumerInfo) Consumer {
	return Consumer{
		Stream: info.Stream, Name: info.Name, Created: info.Created, Durable: info.Config.Durable,
		FilterSubject: info.Config.FilterSubject, FilterSubjects: append([]string(nil), info.Config.FilterSubjects...),
		Mode: consumerMode(info.Config.DeliverSubject), DeliverPolicy: deliverPolicyLabel(info.Config.DeliverPolicy),
		AckPolicy: ackPolicyLabel(info.Config.AckPolicy), AckWaitNanos: info.Config.AckWait.Nanoseconds(),
		MaxDeliver: info.Config.MaxDeliver, ReplayPolicy: replayPolicyLabel(info.Config.ReplayPolicy),
		Metadata: cloneMetadata(info.Config.Metadata),
		Pending:  info.NumPending, AckPending: info.NumAckPending, Redelivered: info.NumRedelivered,
		Waiting: info.NumWaiting, Delivered: info.Delivered.Consumer, Cluster: clusterFromInfo(info.Cluster),
	}
}

func clusterFromInfo(info *jsapi.ClusterInfo) *ClusterInfo {
	if info == nil {
		return nil
	}
	cluster := &ClusterInfo{Name: info.Name, Leader: info.Leader, Replicas: make([]Replica, 0, len(info.Replicas))}
	for _, replica := range info.Replicas {
		cluster.Replicas = append(cluster.Replicas, Replica{
			Name: replica.Name, Current: replica.Current, Offline: replica.Offline,
			Lag: replica.Lag, ActiveNanos: replica.Active.Nanoseconds(),
		})
	}
	return cluster
}

func ptr[T any](value T) *T { return &value }

func storageLabel(storage jsapi.StorageType) string {
	if storage == jsapi.MemoryStorage {
		return "memory"
	}
	return "file"
}

func ackPolicyLabel(policy jsapi.AckPolicy) string {
	switch policy {
	case jsapi.AckNonePolicy:
		return "none"
	case jsapi.AckAllPolicy:
		return "all"
	default:
		return "explicit"
	}
}

func retentionLabel(policy jsapi.RetentionPolicy) string {
	switch policy {
	case jsapi.WorkQueuePolicy:
		return "workqueue"
	case jsapi.InterestPolicy:
		return "interest"
	default:
		return "limits"
	}
}

func discardLabel(policy jsapi.DiscardPolicy) string {
	if policy == jsapi.DiscardNew {
		return "new"
	}
	return "old"
}

func consumerMode(deliverSubject string) string {
	if deliverSubject != "" {
		return "push"
	}
	return "pull"
}

func deliverPolicyLabel(policy jsapi.DeliverPolicy) string {
	switch policy {
	case jsapi.DeliverLastPolicy:
		return "last"
	case jsapi.DeliverNewPolicy:
		return "new"
	case jsapi.DeliverByStartSequencePolicy:
		return "by_start_sequence"
	case jsapi.DeliverByStartTimePolicy:
		return "by_start_time"
	case jsapi.DeliverLastPerSubjectPolicy:
		return "last_per_subject"
	default:
		return "all"
	}
}

func replayPolicyLabel(policy jsapi.ReplayPolicy) string {
	if policy == jsapi.ReplayOriginalPolicy {
		return "original"
	}
	return "instant"
}

func cloneMetadata(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func (c *Client) ServerURL() string { return c.conn.ConnectedUrl() }

func (c *Client) Close() {
	if err := c.conn.Drain(); err != nil {
		c.conn.Close()
	}
}
