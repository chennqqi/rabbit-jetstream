package api

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	prometheusbackend "github.com/chennqqi/rabbit-jetstream/management/internal/prometheus"
)

// The application previously stored an unconfigured Prometheus client as a
// typed nil inside the AlertBackend/HistoryBackend interface fields, so the
// `h.alerts == nil` guard never fired and the alert watcher panicked the
// process (prometheus.(*Client).AlertRules dereferencing a nil receiver).
// These tests pin both layers: the HTTP endpoint must fail safely and the
// background watcher must survive even if a typed nil ever reaches the fields.
func TestOperationalAlertsWithTypedNilBackend(t *testing.T) {
	h := &Handler{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), auth: AuthConfig{OperatorTokens: tokenList("operator")}, alerts: (*prometheusbackend.Client)(nil)}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/v1/alerts", nil)
	request.Header.Set("Authorization", "Bearer operator")
	h.operationalAlerts(recorder, request)
	if recorder.Code != 503 {
		t.Fatalf("status=%d body=%s, want 503 alerts_unavailable", recorder.Code, recorder.Body)
	}
}

func TestMetricHistoryWithTypedNilBackend(t *testing.T) {
	h := &Handler{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), auth: AuthConfig{OperatorTokens: tokenList("operator")}, history: (*prometheusbackend.Client)(nil)}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/v1/history?metric=queue-messages&window=15m&queue=orders", nil)
	request.Header.Set("Authorization", "Bearer operator")
	h.metricHistory(recorder, request)
	if recorder.Code != 503 {
		t.Fatalf("status=%d body=%s, want 503 history_unavailable", recorder.Code, recorder.Body)
	}
}

func TestAlertWatcherSurvivesTypedNilBackend(t *testing.T) {
	h := &Handler{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), events: newEventHub(), eventContext: context.Background(), alerts: (*prometheusbackend.Client)(nil)}
	h.startAlertWatcher()
	// watchAlerts reads the backend on its first loop iteration; a panic here
	// kills the whole test process, so surviving this sleep is the assertion.
	time.Sleep(300 * time.Millisecond)
}
