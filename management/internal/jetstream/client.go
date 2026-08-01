package jetstream

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

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

func streamFromInfo(info *jsapi.StreamInfo) Stream {
	return Stream{
		Name: info.Config.Name, Subjects: info.Config.Subjects, Storage: storageLabel(info.Config.Storage),
		Replicas: info.Config.Replicas, Created: info.Created, Messages: info.State.Msgs,
		Bytes: info.State.Bytes, Consumers: info.State.Consumers, FirstSeq: info.State.FirstSeq,
		LastSeq: info.State.LastSeq, Cluster: clusterFromInfo(info.Cluster),
	}
}

func consumerFromInfo(info *jsapi.ConsumerInfo) Consumer {
	return Consumer{
		Stream: info.Stream, Name: info.Name, Created: info.Created, Durable: info.Config.Durable,
		FilterSubject: info.Config.FilterSubject, AckPolicy: ackPolicyLabel(info.Config.AckPolicy),
		Pending: info.NumPending, AckPending: info.NumAckPending, Redelivered: info.NumRedelivered,
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

func (c *Client) ServerURL() string { return c.conn.ConnectedUrl() }

func (c *Client) Close() {
	if err := c.conn.Drain(); err != nil {
		c.conn.Close()
	}
}
