package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	prometheusbackend "github.com/chennqqi/rabbit-jetstream/management/internal/prometheus"
)

type fakeHistory struct {
	metric prometheusbackend.MetricID
	queue  string
	window prometheusbackend.Window
	result prometheusbackend.Range
	err    error
}

func (f *fakeHistory) QueryRange(_ context.Context, metric prometheusbackend.MetricID, queue string, window prometheusbackend.Window, _ time.Time) (prometheusbackend.Range, error) {
	f.metric, f.queue, f.window = metric, queue, window
	return f.result, f.err
}

func TestHistoryAPIRequiresIdentityAndPassesOnlyTypedQuery(t *testing.T) {
	history := &fakeHistory{result: prometheusbackend.Range{Schema: "rjs.metric-history.v1", Source: "prometheus", Metric: prometheusbackend.MetricQueueMessages, Window: prometheusbackend.Window15Minutes, Series: []prometheusbackend.Series{}}}
	handler := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{OperatorTokens: []string{"operator"}, AuditorTokens: []string{"auditor"}}, ConsoleConfig{History: history})
	if closer, ok := handler.(interface{ Close() }); ok {
		defer closer.Close()
	}
	for _, token := range []string{"operator", "auditor"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/history?metric=queue-messages&window=15m&queue=orders", nil)
		request.Header.Set("Authorization", "Bearer "+token)
		handler.ServeHTTP(recorder, request)
		if recorder.Code != 200 || !strings.Contains(recorder.Body.String(), `"source":"prometheus"`) {
			t.Fatalf("token=%s status=%d body=%s", token, recorder.Code, recorder.Body.String())
		}
	}
	if history.metric != prometheusbackend.MetricQueueMessages || history.queue != "orders" || history.window != prometheusbackend.Window15Minutes {
		t.Fatalf("query=%#v", history)
	}
	for _, path := range []string{"/api/v1/history?metric=queue-messages&window=15m&queue=orders&query=up", "/api/v1/history?metric=x&metric=y&window=15m"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer operator")
		handler.ServeHTTP(recorder, request)
		if recorder.Code != 400 {
			t.Fatalf("path=%s status=%d body=%s", path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestHistoryAPIDisabledAndFailureAreDistinct(t *testing.T) {
	for _, test := range []struct {
		history HistoryBackend
		status  int
		code    string
	}{{nil, 404, "history_api_disabled"}, {&fakeHistory{err: errors.New("private upstream details")}, 503, "history_unavailable"}} {
		handler := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{OperatorTokens: []string{"operator"}}, ConsoleConfig{History: test.history})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/history?metric=management-uptime-seconds&window=1h", nil)
		request.Header.Set("Authorization", "Bearer operator")
		handler.ServeHTTP(recorder, request)
		if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), test.code) || strings.Contains(recorder.Body.String(), "private upstream") {
			t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
		}
		handler.(interface{ Close() }).Close()
	}
}
