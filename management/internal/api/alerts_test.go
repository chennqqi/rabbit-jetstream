package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	prometheusbackend "github.com/chennqqi/rabbit-jetstream/management/internal/prometheus"
)

type fakeAlerts struct{ err error }

func (f fakeAlerts) AlertRules(_ context.Context, now time.Time) (prometheusbackend.AlertSnapshot, error) {
	return prometheusbackend.AlertSnapshot{Schema: "rjs.operational-alerts.v1", Source: "prometheus", ObservedAt: now, Rules: []prometheusbackend.AlertRule{}}, f.err
}

func TestOperationalAlertsAuthorizationAndFailures(t *testing.T) {
	for _, test := range []struct {
		name, path, token string
		backend           AlertBackend
		want              int
	}{
		{"success", "/api/v1/alerts", "ops", fakeAlerts{}, 200}, {"requires auth", "/api/v1/alerts", "", fakeAlerts{}, 401}, {"rejects query", "/api/v1/alerts?rule=x", "ops", fakeAlerts{}, 400}, {"disabled", "/api/v1/alerts", "ops", nil, 404}, {"unavailable", "/api/v1/alerts", "ops", fakeAlerts{err: errors.New("private upstream")}, 503},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := NewWithControllerAuth(&fakeBackend{}, slog.Default(), "test", "dev", nil, nil, AuthConfig{OperatorTokens: []string{"ops"}}, ConsoleConfig{Alerts: test.backend})
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if test.name == "unavailable" {
				body := response.Body.String()
				if body == "" || body == "private upstream" {
					t.Fatalf("unsafe body=%q", body)
				}
			}
		})
	}
}
