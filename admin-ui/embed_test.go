package adminui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesConsoleAndAssets(t *testing.T) {
	for _, test := range []struct{ path, content string }{
		{"/admin/", "Rabbit JetStream"},
		{"/admin/styles.css", "--ink"},
		{"/admin/app.js", "loadDashboard"},
		{"/admin/queues/example", "Rabbit JetStream"},
	} {
		recorder := httptest.NewRecorder()
		Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), test.content) {
			t.Fatalf("%s: status=%d body=%q", test.path, recorder.Code, recorder.Body.String())
		}
		if recorder.Header().Get("Content-Security-Policy") == "" {
			t.Fatalf("%s: missing CSP", test.path)
		}
	}
}

func TestHandlerRejectsTraversal(t *testing.T) {
	recorder := httptest.NewRecorder()
	Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/..%2fsecret", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d", recorder.Code)
	}
}
