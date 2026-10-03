package api

import (
	"bytes"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

func TestAuthenticatedQueueSchema(t *testing.T) {
	for _, tc := range []struct {
		name, token, query, condition string
		auth                          AuthConfig
		status                        int
	}{
		{"operator", "operator", "", "", AuthConfig{OperatorTokens: []string{"operator"}}, 200},
		{"auditor", "auditor", "", "", AuthConfig{AuditorTokens: []string{"auditor"}}, 200},
		{"anonymous", "", "", "", AuthConfig{OperatorTokens: []string{"operator"}}, 401},
		{"disabled", "", "", "", AuthConfig{}, 404},
		{"query", "operator", "?schema=future", "", AuthConfig{OperatorTokens: []string{"operator"}}, 400},
		{"mismatch", "operator", "", `"rjs-capabilities-v1:` + strings.Repeat("0", 64) + `"`, AuthConfig{OperatorTokens: []string{"operator"}}, 412},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewWithControllerAuth(nil, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, tc.auth)
			r := httptest.NewRequest("GET", "/api/v1/console/queue-schema"+tc.query, nil)
			if tc.token != "" {
				r.Header.Set("Authorization", "Bearer "+tc.token)
			}
			if tc.condition != "" {
				r.Header.Set(capabilitiesMatchHeader, tc.condition)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			if tc.status == 200 && (!bytes.Equal(w.Body.Bytes(), topology.QueueSchema()) || w.Header().Get("ETag") != topology.QueueSchemaETag() || w.Header().Get("Content-Type") != "application/schema+json") {
				t.Fatal("schema response drift")
			}
		})
	}
}
