package api

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	prometheusbackend "github.com/chennqqi/rabbit-jetstream/management/internal/prometheus"
)

type changingAlerts struct {
	mu    sync.Mutex
	calls int
}

func (backend *changingAlerts) AlertRules(_ context.Context, now time.Time) (prometheusbackend.AlertSnapshot, error) {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	backend.calls++
	state := "inactive"
	if backend.calls > 1 {
		state = "firing"
	}
	return prometheusbackend.AlertSnapshot{Schema: "rjs.operational-alerts.v1", Source: "prometheus", ObservedAt: now, Rules: []prometheusbackend.AlertRule{{Name: "rule", State: state}}}, nil
}

func TestEventHubTenantReplayAndBudgets(t *testing.T) {
	hub := newEventHub()
	hub.publish("alpha", "audit")
	hub.publish("beta", "audit")
	hub.publish("alpha", "audit")
	fresh, _, freshCancel, err := hub.subscribe("fresh", "alpha", 0, false)
	if err != nil || len(fresh) != 0 {
		t.Fatalf("fresh replay=%+v err=%v", fresh, err)
	}
	freshCancel()
	replay, stream, cancel, err := hub.subscribe("actor", "alpha", 1, true)
	if err != nil || len(replay) != 1 || replay[0].ID != 2 || replay[0].Resource != "audit" {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	defer cancel()
	hub.publish("beta", "audit")
	select {
	case event := <-stream:
		t.Fatalf("cross-tenant event leaked: %+v", event)
	default:
	}
	hub.publish("alpha", "audit")
	select {
	case event := <-stream:
		if event.ID != 3 {
			t.Fatalf("event=%+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("tenant event was not delivered")
	}

	_, _, cancel2, err := hub.subscribe("actor", "alpha", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel2()
	if _, _, _, err = hub.subscribe("actor", "alpha", 0, false); !errors.Is(err, errEventConnectionLimit) {
		t.Fatalf("third actor connection error=%v", err)
	}

	for index := 0; index <= eventReplayLimit+1; index++ {
		hub.publish("gamma", "audit")
	}
	if _, _, _, err = hub.subscribe("other", "gamma", 1, true); !errors.Is(err, errEventReplayExpired) {
		t.Fatalf("expired replay error=%v", err)
	}
}

func TestServerEventsAuthenticatedStream(t *testing.T) {
	handler := &Handler{auth: AuthConfig{OperatorTokens: []string{"ops"}}, events: newEventHub()}
	server := httptest.NewServer(http.HandlerFunc(handler.serverEvents))
	defer server.Close()

	unauthorized, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", unauthorized.StatusCode)
	}
	_ = unauthorized.Body.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	request.Header.Set("Authorization", "Bearer ops")
	request.Header.Set("Accept", "text/event-stream")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") || response.Header.Get("X-Accel-Buffering") != "no" {
		t.Fatalf("status=%d headers=%v", response.StatusCode, response.Header)
	}
	reader := bufio.NewReader(response.Body)
	if line, _ := reader.ReadString('\n'); line != "retry: 3000\n" {
		t.Fatalf("retry line=%q", line)
	}
	if line, _ := reader.ReadString('\n'); line != "\n" {
		t.Fatalf("retry terminator=%q", line)
	}
	handler.events.publish("local", "audit")
	want := []string{"id: 1\n", "event: invalidate\n", "data: {\"resource\":\"audit\"}\n", "\n"}
	for _, expected := range want {
		line, readErr := reader.ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			t.Fatal(readErr)
		}
		if line != expected {
			t.Fatalf("line=%q want=%q", line, expected)
		}
	}
}

func TestServerEventsRejectsInvalidProtocolInputs(t *testing.T) {
	handler := &Handler{auth: AuthConfig{OperatorTokens: []string{"ops"}}, events: newEventHub()}
	for _, test := range []struct {
		name, query, accept, last string
		status                    int
	}{
		{"query", "?x=1", "text/event-stream", "", http.StatusBadRequest},
		{"accept", "", "application/json", "", http.StatusNotAcceptable},
		{"last id", "", "text/event-stream", "zero", http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/events"+test.query, nil)
			request.Header.Set("Authorization", "Bearer ops")
			request.Header.Set("Accept", test.accept)
			request.Header.Set("Last-Event-ID", test.last)
			response := httptest.NewRecorder()
			handler.serverEvents(response, request)
			if response.Code != test.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestServerEventsThroughReverseProxyAndHTTP2(t *testing.T) {
	for _, transport := range []string{"reverse-proxy", "http2"} {
		t.Run(transport, func(t *testing.T) {
			handler := &Handler{auth: AuthConfig{OperatorTokens: []string{"ops"}}, events: newEventHub()}
			var server *httptest.Server
			client := http.DefaultClient
			if transport == "reverse-proxy" {
				upstream := httptest.NewServer(http.HandlerFunc(handler.serverEvents))
				t.Cleanup(upstream.Close)
				target, _ := url.Parse(upstream.URL)
				proxy := httputil.NewSingleHostReverseProxy(target)
				proxy.FlushInterval = -1
				server = httptest.NewServer(proxy)
			} else {
				server = httptest.NewUnstartedServer(http.HandlerFunc(handler.serverEvents))
				server.EnableHTTP2 = true
				server.StartTLS()
				client = server.Client()
			}
			t.Cleanup(server.Close)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			request, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
			request.Header.Set("Authorization", "Bearer ops")
			request.Header.Set("Accept", "text/event-stream")
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if transport == "http2" && response.ProtoMajor != 2 {
				t.Fatalf("protocol=%s", response.Proto)
			}
			reader := bufio.NewReader(response.Body)
			_, _ = reader.ReadString('\n')
			_, _ = reader.ReadString('\n')
			handler.events.publish("local", "audit")
			line, err := reader.ReadString('\n')
			if err != nil || line != "id: 1\n" {
				t.Fatalf("proxied event line=%q err=%v", line, err)
			}
		})
	}
}

func TestEventHubSlowSubscriberIsClosedAndReleased(t *testing.T) {
	hub := newEventHub()
	_, stream, cancel, err := hub.subscribe("slow", "alpha", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	for index := 0; index < 18; index++ {
		hub.publish("alpha", "audit")
	}
	for range stream {
	}
	hub.mu.Lock()
	remaining := len(hub.subscribers)
	hub.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("slow subscriber leaked: %d", remaining)
	}
}

func TestAlertWatcherPublishesOnlyChangedStateToConfiguredTenants(t *testing.T) {
	hub := newEventHub()
	_, alpha, cancelAlpha, err := hub.subscribe("alpha-actor", "alpha", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelAlpha()
	_, beta, cancelBeta, err := hub.subscribe("beta-actor", "beta", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelBeta()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watchAlerts(ctx, &changingAlerts{}, []string{"alpha", "beta"}, hub, time.Millisecond)
	for name, stream := range map[string]<-chan serverEvent{"alpha": alpha, "beta": beta} {
		select {
		case event := <-stream:
			if event.Resource != "alerts" || event.ID != 1 {
				t.Fatalf("%s event=%+v", name, event)
			}
		case <-time.After(time.Second):
			t.Fatalf("%s alert invalidation was not delivered", name)
		}
	}
}
