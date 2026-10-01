package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	contract "github.com/chennqqi/rabbit-jetstream/api"
	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
	"gopkg.in/yaml.v3"
)

type exactConsumerBackend struct {
	*fakeBackend
	calls    int
	deadline bool
}

// Catch model additions, missing fields and optional/required drift. This is
// a field-contract check, not a general JSON Schema validator.
func TestConsumerDetailSchemaFields(t *testing.T) {
	var doc struct {
		Components struct {
			Schemas map[string]struct {
				Required   []string       `yaml:"required"`
				Properties map[string]any `yaml:"properties"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(contract.OpenAPI, &doc); err != nil {
		t.Fatal(err)
	}
	for name, model := range map[string]any{"Consumer": jetstream.Consumer{}, "ClusterInfo": jetstream.ClusterInfo{}, "ConsumerPlan": topology.ConsumerPlan{}, "PlanPreview": jetstream.PlanPreview{}, "ReconcileResult": topology.ReconcileResult{}, "Session": sessionResponse{}} {
		schema := doc.Components.Schemas[name]
		required := map[string]bool{}
		for _, key := range schema.Required {
			required[key] = true
		}
		typ := reflect.TypeOf(model)
		if len(schema.Properties) != typ.NumField() {
			t.Fatalf("%s property count drift", name)
		}
		for i := 0; i < typ.NumField(); i++ {
			tag := strings.Split(typ.Field(i).Tag.Get("json"), ",")
			key := tag[0]
			if _, ok := schema.Properties[key]; !ok {
				t.Errorf("%s missing property %s", name, key)
			}
			optional := len(tag) > 1 && tag[1] == "omitempty"
			if required[key] == optional {
				t.Errorf("%s.%s required mismatch", name, key)
			}
		}
	}
}

func (f *exactConsumerBackend) ListConsumers(context.Context, string) ([]jetstream.Consumer, error) {
	panic("detail must not enumerate a list")
}

func (f *exactConsumerBackend) Consumer(ctx context.Context, stream, name string) (*jetstream.Consumer, error) {
	f.calls++
	deadline, ok := ctx.Deadline()
	f.deadline = ok && time.Until(deadline) > 0 && time.Until(deadline) <= 3*time.Second
	return f.fakeBackend.Consumer(ctx, stream, name)
}

func TestConsumerDetail(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		err        error
		status     int
		code       string
	}{
		{"exact", "/api/v1/streams/orders/consumers/worker", nil, 200, ""},
		{"independent of list page", "/api/v1/streams/orders/consumers/worker?offset=200&limit=1", nil, 200, ""},
		{"wrong stream", "/api/v1/streams/other/consumers/worker", nil, 404, "not_found"},
		{"wrong consumer", "/api/v1/streams/orders/consumers/missing", nil, 404, "not_found"},
		{"unavailable", "/api/v1/streams/orders/consumers/worker", errors.New("offline"), 503, "jetstream_unavailable"},
		{"timeout", "/api/v1/streams/orders/consumers/worker", context.DeadlineExceeded, 503, "jetstream_unavailable"},
		{"canceled", "/api/v1/streams/orders/consumers/worker", context.Canceled, 503, "jetstream_unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := &exactConsumerBackend{fakeBackend: &fakeBackend{err: tc.err, consumers: []jetstream.Consumer{
				{Stream: "another", Name: "worker", Pending: 999},
				{Stream: "orders", Name: "worker", Mode: "push", Pending: 42},
			}}}
			handler := New(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if recorder.Code != tc.status {
				t.Fatalf("status = %d: %s", recorder.Code, recorder.Body.String())
			}
			if backend.calls != 1 || !backend.deadline {
				t.Fatalf("calls=%d deadline=%v", backend.calls, backend.deadline)
			}
			if recorder.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("detail must not be cached")
			}
			if tc.status == 200 {
				var got jetstream.Consumer
				if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got.Stream != "orders" || got.Name != "worker" || got.Pending != 42 || got.Mode != "push" {
					t.Fatalf("wrong identity/metrics: %+v", got)
				}
			} else {
				var got struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got.Error.Code != tc.code {
					t.Fatalf("error code = %q", got.Error.Code)
				}
			}
			if backend.applyCalls != 0 || backend.deleteCalls != 0 || backend.auditCalls != 0 {
				t.Fatal("read produced side effects")
			}
		})
	}
}
