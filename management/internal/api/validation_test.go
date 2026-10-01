package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQueueValidationIssuesBeforeBackend(t *testing.T) {
	for _, method := range []string{"POST", "PUT"} {
		for _, semantic := range []bool{true, false} {
			body := `{"apiVersion":"rabbit-jetstream.io/v1alpha1","kind":"Queue","metadata":{"name":"orders"},"spec":{"subjects":["z","a..b"],"replicas":1}}`
			if !semantic {
				body = "invalid"
			}
			backend := &previewBackend{fakeBackend: &fakeBackend{}}
			handler := NewWithControllerAuth(backend, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{OperatorTokens: []string{"operator"}})
			path := "/api/v1/queues/orders"
			if method == "POST" {
				path += "/preview"
			}
			request := httptest.NewRequest(method, path, strings.NewReader(body))
			request.Header.Set("Authorization", "Bearer operator")
			request.Header.Set("If-None-Match", "*")
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != 400 || backend.calls != 0 || backend.applyCalls != 0 || backend.auditCalls != 0 {
				t.Fatalf("validation reached backend: %d %s", recorder.Code, recorder.Body.String())
			}
			var response struct {
				Error struct {
					Code          string
					IssuesVersion string `json:"issues_version"`
					Issues        []struct{ Path, Code, Message string }
				}
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Error.Code != "invalid_queue" {
				t.Fatal(response)
			}
			if semantic {
				if response.Error.IssuesVersion != "rjs.queue-validation.v1" || len(response.Error.Issues) != 1 || response.Error.Issues[0].Path != "/spec/subjects/1" {
					t.Fatal(response)
				}
			} else if response.Error.IssuesVersion != "" || len(response.Error.Issues) != 0 {
				t.Fatal(response)
			}
		}
	}
}
