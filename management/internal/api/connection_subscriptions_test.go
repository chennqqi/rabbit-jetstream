package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chennqqi/rabbit-jetstream/management/internal/monitoring"
)

type subscriptionMonitorFake struct {
	*connectionFake
	subscriptionCalls int
	err               error
}

func (f *subscriptionMonitorFake) NodeConnectionSubscriptions(ctx context.Context, node string, cid uint64) (*monitoring.ConnectionSubscriptions, error) {
	f.subscriptionCalls++
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 5*time.Second {
		panic("missing bounded deadline")
	}
	if f.err != nil {
		return nil, f.err
	}
	now := time.Now().UTC()
	return &monitoring.ConnectionSubscriptions{NodeID: node, CID: cid, ObservedAt: now, ReadAt: now, Items: []monitoring.ConnectionSubscription{{SID: "001", Subject: "orders.*", Messages: 0}}}, nil
}

func TestConnectionSubscriptionsAPIAuthValidationAndProjection(t *testing.T) {
	for _, token := range []string{"", "auditor", "operator"} {
		fake := &subscriptionMonitorFake{connectionFake: &connectionFake{}}
		h := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", fake, nil, AuthConfig{RequireReadAuth: true, OperatorTokens: []string{"operator"}, AuditorTokens: []string{"auditor"}})
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/v1/nodes/N/connections/18446744073709551615/subscriptions", nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		h.ServeHTTP(w, r)
		if token == "" {
			if w.Code != 401 || fake.subscriptionCalls != 0 {
				t.Fatal("anonymous request reached monitoring")
			}
			continue
		}
		if w.Code != 200 || fake.subscriptionCalls != 1 || !strings.Contains(w.Body.String(), `"cid":18446744073709551615`) || !strings.Contains(w.Body.String(), `"sid":"001"`) {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	}
	for _, target := range []string{
		"/api/v1/nodes/N/connections/0/subscriptions",
		"/api/v1/nodes/N/connections/+1/subscriptions",
		"/api/v1/nodes/N/connections/1/subscriptions?offset=0",
		"/api/v1/nodes/N/connections/18446744073709551616/subscriptions",
	} {
		fake := &subscriptionMonitorFake{connectionFake: &connectionFake{}}
		h := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", fake, nil, AuthConfig{})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", target, nil))
		if w.Code != 400 || fake.subscriptionCalls != 0 {
			t.Fatalf("invalid target %q reached monitoring", target)
		}
	}
}

func TestConnectionSubscriptionsAPIErrors(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
		code   string
	}{
		{monitoring.ErrConnectionMissing, 404, "connection_not_found"},
		{monitoring.ErrConnectionNodeMissing, 404, "not_found"},
		{monitoring.ErrConnectionNodeAmbiguous, 409, "node_identity_ambiguous"},
		{monitoring.ErrConnectionSubscriptionLimit, 422, "subscription_limit_exceeded"},
		{errors.New("credential=secret"), 503, "connections_unavailable"},
	} {
		fake := &subscriptionMonitorFake{connectionFake: &connectionFake{}, err: test.err}
		h := NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", fake, nil, AuthConfig{})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/nodes/N/connections/7/subscriptions", nil))
		if w.Code != test.status || !strings.Contains(w.Body.String(), `"code":"`+test.code+`"`) || strings.Contains(w.Body.String(), "secret") {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	}
}
