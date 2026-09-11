package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

func TestQueueEditableDocument(t *testing.T) {
	for _, scenario := range []string{"valid", "legacy", "revision mismatch"} {
		t.Run(scenario, func(t *testing.T) {
			queue, err := topology.ParseQueue(strings.NewReader(`{"apiVersion":"rabbit-jetstream.io/v1alpha1","kind":"Queue","metadata":{"name":"orders","labels":{"owner":"team"}},"spec":{"subjects":["orders.events"],"replicas":3,"maxPriority":0,"retention":{"maxMessages":9223372036854775807}}}`))
			if err != nil {
				t.Fatal(err)
			}
			plan, err := topology.BuildPlan(*queue)
			if err != nil {
				t.Fatal(err)
			}
			declaration := topology.Declaration{Queue: "orders", Plan: plan, Revision: plan.Revision, KVRevision: 9007199254740993}
			if scenario == "legacy" {
				declaration.Plan.DeclarationSubjects = nil
			}
			if scenario == "revision mismatch" {
				declaration.Revision = "stale"
			}
			backend := &fakeBackend{declarations: []topology.Declaration{declaration}}
			handler := New(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/v1/queues/orders", nil))
			if recorder.Code != 200 || recorder.Header().Get("ETag") != `"9007199254740993"` {
				t.Fatalf("status=%d ETag=%s", recorder.Code, recorder.Header().Get("ETag"))
			}
			var got struct {
				Queue    string
				Plan     topology.Plan
				Document json.RawMessage `json:"document"`
				Error    string          `json:"document_error"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Queue != "orders" || got.Plan.Queue != "orders" {
				t.Fatal("existing fields lost")
			}
			if scenario == "valid" {
				if got.Error != "" {
					t.Fatal(got.Error)
				}
				document, err := topology.ParseQueue(strings.NewReader(string(got.Document)))
				if err != nil {
					t.Fatal(err)
				}
				if document.Spec.MaxPriority == nil || *document.Spec.MaxPriority != 0 || document.Spec.Retention.MaxMessages != 9223372036854775807 || document.Metadata.Labels["owner"] != "team" {
					t.Fatalf("lossy document=%+v", document)
				}
			} else if string(got.Document) != "null" || got.Error == "" {
				t.Fatal("unrepresentable declaration became editable")
			}
			if backend.applyCalls != 0 || backend.auditCalls != 0 || backend.deleteCalls != 0 {
				t.Fatal("read mutated data")
			}
		})
	}
}
