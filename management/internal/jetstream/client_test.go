package jetstream

import (
	"testing"
	"time"

	jsapi "github.com/nats-io/nats.go/jetstream"
)

func TestStreamFromInfo(t *testing.T) {
	info := &jsapi.StreamInfo{
		Config: jsapi.StreamConfig{
			Name: "ORDERS", Subjects: []string{"orders.>"}, Storage: jsapi.FileStorage, Replicas: 3,
			Retention: jsapi.WorkQueuePolicy, Discard: jsapi.DiscardOld, MaxAge: time.Hour,
			MaxBytes: 1024, MaxMsgs: 100, Metadata: map[string]string{"owner": "rjs"},
		},
		State: jsapi.StreamState{Msgs: 12, Bytes: 1024, Consumers: 2, FirstSeq: 4, LastSeq: 15},
		Cluster: &jsapi.ClusterInfo{Name: "rjs", Leader: "nats-1", Replicas: []*jsapi.PeerInfo{
			{Name: "nats-2", Current: true, Active: time.Second, Lag: 0},
		}},
	}
	stream := streamFromInfo(info)
	if stream.Name != "ORDERS" || stream.Storage != "file" || stream.Replicas != 3 || stream.Messages != 12 || stream.Retention != "workqueue" || stream.MaxAgeNanos != int64(time.Hour) {
		t.Fatalf("unexpected stream: %#v", stream)
	}
	if stream.Cluster == nil || stream.Cluster.Leader != "nats-1" || !stream.Cluster.Replicas[0].Current {
		t.Fatalf("unexpected cluster: %#v", stream.Cluster)
	}
}

func TestConsumerFromInfo(t *testing.T) {
	info := &jsapi.ConsumerInfo{
		Stream: "ORDERS", Name: "WORKERS",
		Config: jsapi.ConsumerConfig{
			Durable: "WORKERS", FilterSubjects: []string{"orders.created", "orders.updated"},
			AckPolicy: jsapi.AckExplicitPolicy, AckWait: 30 * time.Second, MaxDeliver: 5,
			DeliverPolicy: jsapi.DeliverAllPolicy, ReplayPolicy: jsapi.ReplayInstantPolicy,
		},
		Delivered: jsapi.SequenceInfo{Consumer: 7}, NumPending: 5, NumAckPending: 2, NumRedelivered: 1,
	}
	consumer := consumerFromInfo(info)
	if consumer.Name != "WORKERS" || consumer.Mode != "pull" || consumer.AckPolicy != "explicit" || consumer.Delivered != 7 || consumer.Pending != 5 || consumer.MaxDeliver != 5 || len(consumer.FilterSubjects) != 2 {
		t.Fatalf("unexpected consumer: %#v", consumer)
	}
}

func TestClusterFromInfoAllowsStandalone(t *testing.T) {
	if cluster := clusterFromInfo(nil); cluster != nil {
		t.Fatalf("cluster = %#v", cluster)
	}
}
