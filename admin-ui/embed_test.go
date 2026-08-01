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
		{"/admin/management.js", "If-None-Match"},
		{"/admin/management.css", ".danger-zone"},
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
		for header, expected := range map[string]string{
			"Cross-Origin-Resource-Policy": "same-origin",
			"Permissions-Policy":           "camera=(), microphone=(), geolocation=()",
			"Referrer-Policy":              "no-referrer",
			"X-Content-Type-Options":       "nosniff",
			"X-Frame-Options":              "DENY",
		} {
			if actual := recorder.Header().Get(header); actual != expected {
				t.Fatalf("%s: %s=%q, want %q", test.path, header, actual, expected)
			}
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

func TestManagementUIPreservesMutationSafetyContract(t *testing.T) {
	script, err := assets.ReadFile("dist/management.js")
	if err != nil {
		t.Fatal(err)
	}
	value := string(script)
	for _, required := range []string{"Authorization", "If-None-Match", "If-Match", "X-RJS-Confirm-Queue", "operator-token", "managedQueueName&&name!==managedQueueName", "cannot be renamed"} {
		if !strings.Contains(value, required) {
			t.Errorf("management UI missing %q", required)
		}
	}
	for _, forbidden := range []string{"localStorage", "sessionStorage", "document.cookie"} {
		if strings.Contains(value, forbidden) {
			t.Errorf("management UI persists bearer credential through %q", forbidden)
		}
	}
}
