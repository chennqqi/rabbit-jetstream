package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

type queueObservationBackend struct {
	*fakeBackend
	streamErr error
}

func (backend *queueObservationBackend) ListStreams(context.Context) ([]jetstream.Stream, error) {
	return nil, backend.streamErr
}

func queueListFixture(name string, replicas int) (topology.Declaration, jetstream.Stream) {
	revision := "revision-" + name
	metadata := map[string]string{"rabbit-jetstream.io/queue": name, "rabbit-jetstream.io/revision": revision}
	plan := topology.StreamPlan{Name: "stream-" + name, Subjects: []string{"b", "a"}, Storage: "file", Replicas: replicas, Retention: "workqueue", Discard: "old", MaxAgeNanos: 2, MaxBytes: 3, MaxMessages: 4, Metadata: metadata}
	declaration := topology.Declaration{Queue: name, Revision: revision, Plan: topology.Plan{Queue: name, Revision: revision, Stream: plan}}
	stream := jetstream.Stream{Name: plan.Name, Subjects: []string{"a", "b"}, Storage: plan.Storage, Replicas: replicas, Retention: plan.Retention, Discard: plan.Discard, MaxAgeNanos: plan.MaxAgeNanos, MaxBytes: plan.MaxBytes, MaxMessages: plan.MaxMessages, Messages: 0, Consumers: 0, Metadata: map[string]string{"rabbit-jetstream.io/queue": name, "rabbit-jetstream.io/revision": revision}}
	if replicas > 1 {
		stream.Cluster = &jetstream.ClusterInfo{Leader: "leader", Replicas: []jetstream.Replica{{Name: "follower-1", Current: true}, {Name: "follower-2", Current: true}}}
	}
	return declaration, stream
}

func TestObserveQueueListStatesAndExactZeroCounts(t *testing.T) {
	consistent, stream := queueListFixture("consistent", 3)
	missing, _ := queueListFixture("missing", 1)
	mismatch, mismatchStream := queueListFixture("mismatch", 1)
	mismatchStream.Metadata["rabbit-jetstream.io/queue"] = "other"
	notCurrent, notCurrentStream := queueListFixture("not-current", 3)
	notCurrentStream.Cluster.Replicas[0].Current = false
	invalid, _ := queueListFixture("invalid", 1)
	invalid.Plan.Revision = "other"

	got := observeQueueList([]topology.Declaration{consistent, missing, mismatch, notCurrent, invalid}, []jetstream.Stream{stream, mismatchStream, notCurrentStream}, nil)
	wantStates := []string{"present", "missing", "degraded", "degraded", "unavailable"}
	wantReasons := [][]string{{}, {"stream_missing"}, {"stream_configuration_mismatch"}, {"stream_replicas_not_current"}, {"declaration_inconsistent"}}
	for index := range got {
		if got[index].Observation.State != wantStates[index] || !reflect.DeepEqual(got[index].Observation.Reasons, wantReasons[index]) {
			t.Fatalf("item %d observation=%+v", index, got[index].Observation)
		}
	}
	if got[0].Observation.Messages == nil || *got[0].Observation.Messages != 0 || got[0].Observation.Consumers == nil || *got[0].Observation.Consumers != 0 {
		t.Fatalf("explicit zero counts lost: %+v", got[0].Observation)
	}
	if got[1].Observation.Messages != nil || got[1].Observation.Consumers != nil {
		t.Fatalf("missing Stream invented counts: %+v", got[1].Observation)
	}
}

func TestObserveQueueListUnavailableAndAmbiguous(t *testing.T) {
	declaration, stream := queueListFixture("orders", 1)
	unavailable := observeQueueList([]topology.Declaration{declaration}, nil, errors.New("private upstream detail"))[0].Observation
	if unavailable.State != "unavailable" || !reflect.DeepEqual(unavailable.Reasons, []string{"stream_collection_unavailable"}) {
		t.Fatalf("unavailable=%+v", unavailable)
	}
	ambiguous := observeQueueList([]topology.Declaration{declaration}, []jetstream.Stream{stream, stream}, nil)[0].Observation
	if ambiguous.State != "unavailable" || !reflect.DeepEqual(ambiguous.Reasons, []string{"stream_identity_ambiguous"}) {
		t.Fatalf("ambiguous=%+v", ambiguous)
	}
}

func TestObserveQueueListDoesNotMutateSourceSubjects(t *testing.T) {
	declaration, stream := queueListFixture("orders", 1)
	beforePlan, beforeStream := append([]string(nil), declaration.Plan.Stream.Subjects...), append([]string(nil), stream.Subjects...)
	_ = observeQueueList([]topology.Declaration{declaration}, []jetstream.Stream{stream}, nil)
	if !reflect.DeepEqual(declaration.Plan.Stream.Subjects, beforePlan) || !reflect.DeepEqual(stream.Subjects, beforeStream) {
		t.Fatal("observation reordered backend-owned subjects")
	}
}

func TestQueueListRetainsDeclarationsWhenStreamCollectionFails(t *testing.T) {
	declaration, _ := queueListFixture("orders", 1)
	backend := &queueObservationBackend{fakeBackend: &fakeBackend{declarations: []topology.Declaration{declaration}}, streamErr: errors.New("nats://user:private@nats.internal:4222")}
	handler := New(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/v1/queues", nil))
	if recorder.Code != 200 || strings.Contains(recorder.Body.String(), "private") || strings.Contains(recorder.Body.String(), "nats.internal") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Items []queueListItem `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 1 || response.Items[0].Queue != "orders" || response.Items[0].Observation.State != "unavailable" || response.Items[0].Observation.Messages != nil {
		t.Fatalf("response=%+v", response)
	}
}
