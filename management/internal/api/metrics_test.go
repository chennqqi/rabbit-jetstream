package api

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/chennqqi/rabbit-jetstream/management/internal/controller"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
	"github.com/chennqqi/rabbit-jetstream/management/internal/monitoring"
)

func TestPrometheusMetricsExposeRuntimeAndRequestState(t *testing.T) {
	backend := &fakeBackend{
		account:      jetstream.Account{MemoryUsed: 10, StorageUsed: 20, Streams: 1, Consumers: 1, APIErrors: 2},
		streams:      []jetstream.Stream{{Name: "RJSQ_orders", Messages: 7, Bytes: 99, Metadata: map[string]string{"rabbit-jetstream.io/queue": "orders"}}},
		declarations: []topology.Declaration{{Queue: "orders"}},
	}
	monitor := fakeMonitor{snapshot: monitoring.Snapshot{Status: "available", Total: 1, Available: 1}}
	control := fakeController{status: controller.Status{InstanceID: "node\"1", Leader: true, Blocked: 2, DLQMoved: 3, DLQFailed: 1, LastSuccess: time.Unix(100, 0)}}
	handler := NewWithController(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "rjs", "v1", monitor, control, "")

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := recorder.Body.String()
	for _, expected := range []string{
		`rjs_build_info{name="rjs",version="v1"} 1`,
		`rjs_jetstream_up 1`, `rjs_jetstream_storage_bytes 20`, `rjs_queues_declared 1`,
		`rjs_queue_messages{queue="orders"} 7`, `rjs_nats_nodes{status="available"} 1`,
		`rjs_controller_leader{instance="node\"1"} 1`, `rjs_controller_blocked_queues{instance="node\"1"} 2`,
		`rjs_dlq_moved_total{instance="node\"1"} 3`, `rjs_controller_last_success_timestamp_seconds{instance="node\"1"} 100`,
		`rjs_http_requests_total{code="200",method="GET",route="GET /healthz"} 1`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in:\n%s", expected, body)
		}
	}
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("status=%d headers=%v", recorder.Code, recorder.Header())
	}
}

func TestPrometheusMetricsReportJetStreamDown(t *testing.T) {
	handler := newTestHandler(&fakeBackend{err: errors.New("unavailable")})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(recorder.Body.String(), "rjs_jetstream_up 0") {
		t.Fatalf("body=%s", recorder.Body.String())
	}
}
