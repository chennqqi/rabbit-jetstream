package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	for _, test := range []struct {
		name       string
		statusCode int
		wantError  string
	}{
		{name: "healthy", statusCode: http.StatusOK},
		{name: "unhealthy", statusCode: http.StatusServiceUnavailable, wantError: "503 Service Unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(test.statusCode) }))
			defer server.Close()
			err := check(server.URL)
			if test.wantError == "" && err != nil {
				t.Fatalf("check returned error: %v", err)
			}
			if test.wantError != "" && (err == nil || !strings.Contains(err.Error(), test.wantError)) {
				t.Fatalf("check error = %v, want substring %q", err, test.wantError)
			}
		})
	}
}
