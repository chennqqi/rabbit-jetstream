package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

type globalHTTPBackend struct {
	*fakeBackend
	byStream         map[string][]jetstream.Consumer
	declarationCalls int
}

func (backend *globalHTTPBackend) ListConsumers(_ context.Context, stream string) ([]jetstream.Consumer, error) {
	return backend.byStream[stream], backend.err
}
func (backend *globalHTTPBackend) ListDeclarations(context.Context) ([]topology.Declaration, error) {
	backend.declarationCalls++
	return backend.declarations, backend.err
}

func TestGlobalConsumersRequireExplicitAuthorizedRefresh(t *testing.T) {
	plan := topology.Plan{Queue: "q", Stream: topology.StreamPlan{Name: "S"}, Consumer: topology.ConsumerPlan{Name: "C", Stream: "S", Mode: "pull"}}
	backend := &globalHTTPBackend{fakeBackend: &fakeBackend{streams: []jetstream.Stream{{Name: "S"}}, declarations: []topology.Declaration{{Queue: "q", KVRevision: 1, Plan: plan}}}, byStream: map[string][]jetstream.Consumer{"S": {{Stream: "S", Name: "C", Mode: "pull"}}}}
	handler := NewWithControllerAuth(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{RequireReadAuth: true, OperatorTokens: []string{"operator"}, AuditorTokens: []string{"auditor"}})
	request := func(method, path, token string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, path, nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		handler.ServeHTTP(w, r)
		return w
	}
	if got := request(http.MethodGet, "/api/v1/consumers", "auditor"); got.Code != http.StatusServiceUnavailable || backend.declarationCalls != 0 {
		t.Fatalf("initial=%d calls=%d body=%s", got.Code, backend.declarationCalls, got.Body.String())
	}
	if got := request(http.MethodPost, "/api/v1/consumers/refresh", "auditor"); got.Code != http.StatusForbidden || backend.declarationCalls != 0 {
		t.Fatalf("auditor=%d calls=%d", got.Code, backend.declarationCalls)
	}
	if got := request(http.MethodPost, "/api/v1/consumers/refresh", "operator"); got.Code != http.StatusOK || backend.declarationCalls != 2 {
		t.Fatalf("refresh=%d calls=%d body=%s", got.Code, backend.declarationCalls, got.Body.String())
	}
	got := request(http.MethodGet, "/api/v1/consumers?queue=q&limit=1", "auditor")
	if got.Code != http.StatusOK {
		t.Fatalf("list=%d %s", got.Code, got.Body.String())
	}
	raw := got.Body.String()
	if strings.Contains(raw, `"Total"`) || !strings.Contains(raw, `"total":1`) || !strings.Contains(raw, `"offset":0`) || !strings.Contains(raw, `"limit":1`) {
		t.Fatalf("wire keys=%s", raw)
	}
	var page struct {
		State        string              `json:"state"`
		GenerationID string              `json:"generation_id"`
		Total        int                 `json:"total"`
		Items        []globalConsumerRow `json:"items"`
	}
	if err := json.NewDecoder(strings.NewReader(raw)).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if page.State != "ready" || page.GenerationID == "" || page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page=%#v", page)
	}
	if got := request(http.MethodGet, "/api/v1/consumers?generation=old", "auditor"); got.Code != http.StatusConflict {
		t.Fatalf("generation=%d", got.Code)
	}
}

func TestGlobalConsumerInvalidQueriesDoNotReadBackend(t *testing.T) {
	backend := &globalHTTPBackend{fakeBackend: &fakeBackend{}, byStream: map[string][]jetstream.Consumer{}}
	handler := NewWithControllerAuth(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{OperatorTokens: []string{"operator"}})
	for _, path := range []string{"/api/v1/consumers?q=a&q=b", "/api/v1/consumers?mode=bad", "/api/v1/consumers?limit=0", "/api/v1/consumers?unknown=x"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s=%d", path, w.Code)
		}
	}
	if backend.declarationCalls != 0 {
		t.Fatalf("invalid query reached backend: %d", backend.declarationCalls)
	}
}
