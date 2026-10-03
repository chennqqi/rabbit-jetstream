package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/management/internal/controller"
)

func TestControllerStatusReportsIgnoredIncludingZero(t *testing.T) {
	for _, ignored := range []int{0, 7} {
		handler := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil,
			fakeController{status: controller.Status{InstanceID: "one", DLQProcessed: ignored + 3, DLQMoved: 2, DLQFailed: 1, DLQIgnored: ignored}},
			AuthConfig{RequireReadAuth: true, AuditorTokens: []string{"auditor"}})
		request := httptest.NewRequest(http.MethodGet, "/api/v1/controller", nil)
		request.Header.Set("Authorization", "Bearer auditor")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		var body map[string]json.RawMessage
		if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &body) != nil {
			t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
		}
		for key, expected := range map[string]int{"dlqProcessed": ignored + 3, "dlqMoved": 2, "dlqFailed": 1, "dlqIgnored": ignored} {
			var value int
			if err := json.Unmarshal(body[key], &value); err != nil || value != expected {
				t.Fatalf("field %s=%s, want %d", key, body[key], expected)
			}
		}
	}
}
