package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
)

func TestResourceReadAuthorization(t *testing.T) {
	for _, path := range []string{"/api/v1/info", "/api/v1/cluster", "/api/v1/nodes", "/api/v1/queues", "/api/v1/queues/orders", "/api/v1/queues/orders/consumers", "/api/v1/streams", "/api/v1/streams/ORDERS", "/api/v1/streams/ORDERS/consumers", "/api/v1/streams/ORDERS/consumers/worker", "/api/v1/controller"} {
		for _, method := range []string{"GET", "HEAD"} {
			for _, tc := range []struct {
				token  string
				denied bool
			}{{"", true}, {"bad", true}, {"operator", false}, {"auditor", false}} {
				handler := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{RequireReadAuth: true, OperatorTokens: []string{"operator"}, AuditorTokens: []string{"auditor"}})
				request := httptest.NewRequest(method, path, nil)
				request.Header.Set("Authorization", "Bearer "+tc.token)
				request.Header.Set("X-Forwarded-For", "127.0.0.1")
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)
				if (recorder.Code == 401) != tc.denied {
					t.Fatalf("%s %s token=%q status=%d", method, path, tc.token, recorder.Code)
				}
			}
		}
	}
}

func TestReadPolicyAndPublicBootstrap(t *testing.T) {
	handler := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{RequireReadAuth: true, OperatorTokens: []string{"operator"}})
	for _, path := range []string{"/healthz", "/readyz", "/metrics", "/admin/", "/api/v1/openapi.yaml", "/api/v1/native-sdk-contract.json"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest("GET", path, nil))
		if recorder.Code != 200 {
			t.Fatalf("bootstrap %s: %d", path, recorder.Code)
		}
	}
	// The login page reads the browser OIDC config before any credential
	// exists, so it must stay public under required read authentication; the
	// disabled-deployment response stays a 404, not an auth failure.
	oidcRecorder := httptest.NewRecorder()
	handler.ServeHTTP(oidcRecorder, httptest.NewRequest("GET", "/api/v1/oidc/config", nil))
	if oidcRecorder.Code != 404 {
		t.Fatalf("unauthenticated oidc/config under required read auth: %d", oidcRecorder.Code)
	}
	request := httptest.NewRequest("GET", "/api/v1/session", nil)
	request.Header.Set("Authorization", "Bearer operator")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	var session sessionResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	if session.ResourceReadPolicy != "authenticated" {
		t.Fatal("incorrect read policy")
	}
	disabled := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{RequireReadAuth: true})
	recorder = httptest.NewRecorder()
	disabled.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/v1/queues", nil))
	if recorder.Code != 404 {
		t.Fatalf("unconfigured read authorization status=%d", recorder.Code)
	}
}

func TestDisabledResourceReadsExposeExactRetentionBoundary(t *testing.T) {
	handler := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{RequireReadAuth: true})
	for _, path := range []string{"/api/v1/info", "/api/v1/queues", "/api/v1/streams", "/api/v1/nodes", "/api/v1/controller"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest("GET", path, nil)
			request.Header.Set("Authorization", "Bearer prior-session-token")
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			var body map[string]struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || recorder.Code != 404 || body["error"].Code != "read_api_disabled" {
				t.Fatalf("status=%d body=%s error=%v", recorder.Code, recorder.Body.String(), err)
			}
		})
	}
}
