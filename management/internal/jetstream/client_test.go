package jetstream

import (
	"testing"
	"time"

	jsapi "github.com/nats-io/nats.go/jetstream"
)

func TestStreamFromInfo(t *testing.T) {
	info := &jsapi.StreamInfo{
		Config: jsapi.StreamConfig{Name: "ORDERS", Subjects: []string{"orders.>"}, Storage: jsapi.FileStorage, Replicas: 3},
		State:  jsapi.StreamState{Msgs: 12, Bytes: 1024, Consumers: 2, FirstSeq: 4, LastSeq: 15},
		Cluster: &jsapi.ClusterInfo{Name: "rjs", Leader: "nats-1", Replicas: []*jsapi.PeerInfo{
			{Name: "nats-2", Current: true, Active: time.Second, Lag: 0},
		}},
	}
	stream := streamFromInfo(info)
	if stream.Name != "ORDERS" || stream.Storage != "file" || stream.Replicas != 3 || stream.Messages != 12 {
		t.Fatalf("unexpected stream: %#v", stream)
	}
	if stream.Cluster == nil || stream.Cluster.Leader != "nats-1" || !stream.Cluster.Replicas[0].Current {
		t.Fatalf("unexpected cluster: %#v", stream.Cluster)
	}
}

func TestConsumerFromInfo(t *testing.T) {
	info := &jsapi.ConsumerInfo{
		Stream: "ORDERS", Name: "WORKERS",
		Config:    jsapi.ConsumerConfig{Durable: "WORKERS", FilterSubject: "orders.created", AckPolicy: jsapi.AckExplicitPolicy},
		Delivered: jsapi.SequenceInfo{Consumer: 7}, NumPending: 5, NumAckPending: 2, NumRedelivered: 1,
	}
	consumer := consumerFromInfo(info)
	if consumer.Name != "WORKERS" || consumer.AckPolicy != "explicit" || consumer.Delivered != 7 || consumer.Pending != 5 {
		t.Fatalf("unexpected consumer: %#v", consumer)
	}
}

func TestClusterFromInfoAllowsStandalone(t *testing.T) {
	if cluster := clusterFromInfo(nil); cluster != nil {
		t.Fatalf("cluster = %#v", cluster)
	}
}
