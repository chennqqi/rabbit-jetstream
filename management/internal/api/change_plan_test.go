package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

type changePlanBackend struct {
	fakeBackend
	calls []string
}

func (f *changePlanBackend) Preview(ctx context.Context, plan topology.Plan, condition jetstream.ApplyPrecondition) (*jetstream.PlanPreview, error) {
	if _, ok := ctx.Deadline(); !ok || condition.ExpectedRevision == nil || condition.CreateOnly {
		panic("change preview lost its bounded update precondition")
	}
	f.calls = append(f.calls, plan.Queue)
	switch plan.Queue {
	case "conflict":
		return nil, jetstream.ErrConflict
	case "missing":
		return nil, jetstream.ErrNotFound
	case "offline":
		return nil, errors.New("private backend detail")
	case "cycle":
		return nil, jetstream.ErrDeadLetterCycle
	}
	return &jetstream.PlanPreview{Plan: plan, Result: topology.ReconcileResult{}, BaseRevision: `"` + strconv.FormatUint(*condition.ExpectedRevision, 10) + `"`}, nil
}

func changePlanHandler(f *changePlanBackend) http.Handler {
	return NewWithControllerAuth(f, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{OperatorTokens: []string{"operator"}, AuditorTokens: []string{"auditor"}})
}

func changeDocument(name string) json.RawMessage { return importDocument(name, "") }

func requestChangePlan(t *testing.T, handler http.Handler, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("POST", "/api/v1/queues/change-plan", strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestQueueChangePlanItemizedFailuresAndNoWrites(t *testing.T) {
	f := &changePlanBackend{}
	items := []map[string]any{
		{"document": changeDocument("ready"), "etag": `"7"`},
		{"document": changeDocument("conflict"), "etag": `"8"`},
		{"document": changeDocument("missing"), "etag": `"9"`},
		{"document": changeDocument("offline"), "etag": `"10"`},
		{"document": changeDocument("cycle"), "etag": `"13"`},
		{"document": changeDocument("bad-etag"), "etag": `"01"`},
		{"document": changeDocument("duplicate"), "etag": `"11"`},
		{"document": changeDocument("duplicate"), "etag": `"12"`},
	}
	raw, _ := json.Marshal(map[string]any{"items": items})
	w := requestChangePlan(t, changePlanHandler(f), string(raw), "operator")
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Scope string                `json:"scope"`
		Ready bool                  `json:"ready"`
		Items []queueChangePlanItem `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Scope != "bulk-change-preview-not-apply-authorization" || got.Ready || len(got.Items) != len(items) {
		t.Fatalf("result=%+v", got)
	}
	want := []string{"ready", "conflict", "missing", "unavailable", "blocked", "invalid", "invalid", "invalid"}
	for index := range want {
		if got.Items[index].Index != index || got.Items[index].Status != want[index] {
			t.Fatalf("item %d=%+v", index, got.Items[index])
		}
	}
	if got.Items[4].Code != "dlq_dependency_cycle" || got.Items[5].Code != "invalid_etag" || got.Items[6].Code != "duplicate_queue" || got.Items[3].Code != "jetstream_unavailable" {
		t.Fatalf("codes=%+v", got.Items)
	}
	if got.Items[0].Preview == nil || got.Items[0].Preview.BaseRevision != `"7"` {
		t.Fatalf("ready preview=%+v", got.Items[0])
	}
	if got.Items[0].Code != "" {
		t.Fatalf("ready item retained error code %q", got.Items[0].Code)
	}
	if strings.Contains(w.Body.String(), "private backend detail") || f.applyCalls != 0 || f.deleteCalls != 0 || f.auditCalls != 0 {
		t.Fatal("planning leaked detail or mutated state")
	}
}

func TestQueueChangePlanEnvelopeAndAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name, body, token string
		status            int
	}{
		{"anonymous", `{"items":[]}`, "", 401}, {"auditor", `{"items":[]}`, "auditor", 403},
		{"empty", `{"items":[]}`, "operator", 400}, {"unknown", `{"items":[],"extra":true}`, "operator", 400},
		{"trailing", `{"items":[{"document":{},"etag":"\"1\""}]} {}`, "operator", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &changePlanBackend{}
			w := requestChangePlan(t, changePlanHandler(f), tc.body, tc.token)
			if w.Code != tc.status || len(f.calls) != 0 {
				t.Fatalf("status=%d calls=%v body=%s", w.Code, f.calls, w.Body.String())
			}
		})
	}
}
