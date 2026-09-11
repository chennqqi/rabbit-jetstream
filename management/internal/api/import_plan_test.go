package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

type importBackend struct {
	fakeBackend
	reads  []string
	values map[string]*topology.Declaration
}

func (f *importBackend) Declaration(ctx context.Context, name string) (*topology.Declaration, error) {
	f.reads = append(f.reads, name)
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 5*time.Second {
		panic("unbounded lookup")
	}
	if value, ok := f.values[name]; ok {
		return value, nil
	}
	return nil, jetstream.ErrNotFound
}
func importDocument(name, dependency string) json.RawMessage {
	q := topology.Queue{APIVersion: topology.QueueAPIVersion, Kind: topology.QueueKind, Metadata: topology.Metadata{Name: name}, Spec: topology.QueueSpec{Replicas: 1, Subjects: []string{name + ".events"}}}
	if dependency != "" {
		q.Spec.DeadLetter = &topology.DeadLetterPolicy{Queue: dependency}
	}
	raw, _ := json.Marshal(q)
	return raw
}
func importHandler(f *importBackend) http.Handler {
	return NewWithControllerAuth(f, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{OperatorTokens: []string{"operator"}, AuditorTokens: []string{"auditor"}})
}

func TestImportPlanAPIIndexedProblemsAndExternalEvidence(t *testing.T) {
	external, err := topology.ParseQueue(strings.NewReader(string(importDocument("outside", ""))))
	if err != nil {
		t.Fatal(err)
	}
	p, err := topology.BuildPlan(*external)
	if err != nil {
		t.Fatal(err)
	}
	f := &importBackend{values: map[string]*topology.Declaration{"outside": {Queue: "outside", Revision: p.Revision, Plan: p, KVRevision: 9007199254740993}}}
	bad := strings.Replace(string(importDocument("bad", "")), `"spec":`, `"unknown":true,"spec":`, 1)
	raw, _ := json.Marshal(map[string]any{"documents": []json.RawMessage{importDocument("source", "outside"), importDocument("second", "outside"), json.RawMessage(bad), importDocument("dependent", "bad"), importDocument("root", ""), importDocument("missing_user", "absent")}})
	r := httptest.NewRequest("POST", "/api/v1/queues/import-plan", strings.NewReader(string(raw)))
	r.Header.Set("Authorization", "Bearer operator")
	w := httptest.NewRecorder()
	importHandler(f).ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var result struct {
		Plan     topology.ImportPlan           `json:"plan"`
		External []importDependencyObservation `json:"external_declarations"`
		Scope    string                        `json:"scope"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Plan.Items) != 6 || result.Plan.Ready || len(result.Plan.Order) != 1 || result.Plan.Order[0] != 4 {
		t.Fatalf("bad plan: %+v", result)
	}
	// Both present and missing external declarations may reach explicit preview;
	// neither changes planning readiness or permits writes.
	if got := fmt.Sprint(result.Plan.ReviewOrder); got != "[5 4 1 0]" {
		t.Fatalf("review order=%s", got)
	}
	if result.Plan.Items[2].Problems[0].Code != "invalid_declaration" || result.Plan.Items[3].Problems[0].Code != "blocked_dependency" {
		t.Fatal("invalid identity lost")
	}
	if len(f.reads) != 2 || f.reads[0] != "absent" || f.reads[1] != "outside" {
		t.Fatalf("non-deduplicated reads: %v", f.reads)
	}
	if result.External[0].Status != "missing" || result.External[1].Status != "present" || result.External[1].ETag != `"9007199254740993"` {
		t.Fatalf("bad observations: %+v", result.External)
	}
	if result.Plan.Items[0].Problems[0].Code != "external_dependency_unverified" {
		t.Fatal("declaration observation incorrectly authorized dependency")
	}
	if f.applyCalls != 0 || f.deleteCalls != 0 || f.auditCalls != 0 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("mutation or cacheable response")
	}
}

func TestImportPlanAPIAuthAndEnvelope(t *testing.T) {
	for _, token := range []string{"", "bad", "auditor", "operator"} {
		f := &importBackend{}
		r := httptest.NewRequest("POST", "/api/v1/queues/import-plan", strings.NewReader(`{"documents":[`+string(importDocument("a", ""))+`]}`))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		importHandler(f).ServeHTTP(w, r)
		want := 401
		if token == "auditor" {
			want = 403
		}
		if token == "operator" {
			want = 200
		}
		if w.Code != want || len(f.reads) != 0 {
			t.Fatalf("token=%s %d %s", token, w.Code, w.Body.String())
		}
	}
	for _, body := range []string{`null`, `[]`, `{}`, `{"documents":[]}`, `{"documents":null}`, `{"documents":[],"documents":[]}`, `{"documents":[],"extra":true}`, `{"documents":[null]} {}`, `{"documents":[` + strings.Repeat(`{},`, 100) + `{}]}`, strings.Repeat(" ", 4<<20) + `{}`} {
		f := &importBackend{}
		r := httptest.NewRequest("POST", "/api/v1/queues/import-plan", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer operator")
		w := httptest.NewRecorder()
		importHandler(f).ServeHTTP(w, r)
		if w.Code != 400 || len(f.reads) != 0 {
			t.Fatalf("bad envelope accepted: %d", w.Code)
		}
	}
}

func TestImportPlanAPICanceledAndUnrepresentable(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		f := &importBackend{values: map[string]*topology.Declaration{"outside": {Queue: "wrong"}}}
		r := httptest.NewRequest("POST", "/api/v1/queues/import-plan", strings.NewReader(`{"documents":[`+string(importDocument("source", "outside"))+`]}`))
		r.Header.Set("Authorization", "Bearer operator")
		if canceled {
			ctx, cancel := context.WithCancel(r.Context())
			cancel()
			r = r.WithContext(ctx)
		}
		w := httptest.NewRecorder()
		importHandler(f).ServeHTTP(w, r)
		if canceled {
			if w.Code != 503 || len(f.reads) != 0 {
				t.Fatal("canceled lookup dispatched")
			}
		} else if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"unrepresentable"`) {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
	}
}
