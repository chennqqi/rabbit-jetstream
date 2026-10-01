package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chennqqi/rabbit-jetstream/management/internal/identity"
	prometheusbackend "github.com/chennqqi/rabbit-jetstream/management/internal/prometheus"
	"github.com/chennqqi/rabbit-jetstream/management/internal/tenant"
)

const (
	eventReplayLimit       = 256
	eventActorConnections  = 2
	eventTenantConnections = 32
	eventTotalConnections  = 256
)

var errEventReplayExpired = errors.New("event replay window expired")

type serverEvent struct {
	ID       uint64 `json:"-"`
	Resource string `json:"resource"`
}

type eventSubscriber struct {
	actor  string
	tenant string
	events chan serverEvent
}

type tenantEventState struct {
	next   uint64
	replay []serverEvent
}

type eventHub struct {
	mu          sync.Mutex
	tenants     map[string]*tenantEventState
	subscribers map[*eventSubscriber]struct{}
}

func newEventHub() *eventHub {
	return &eventHub{tenants: map[string]*tenantEventState{}, subscribers: map[*eventSubscriber]struct{}{}}
}

func (hub *eventHub) publish(tenantID, resource string) {
	if hub == nil || tenantID == "" {
		return
	}
	hub.mu.Lock()
	defer hub.mu.Unlock()
	state := hub.tenants[tenantID]
	if state == nil {
		state = &tenantEventState{}
		hub.tenants[tenantID] = state
	}
	state.next++
	event := serverEvent{ID: state.next, Resource: resource}
	state.replay = append(state.replay, event)
	if len(state.replay) > eventReplayLimit {
		state.replay = append([]serverEvent(nil), state.replay[len(state.replay)-eventReplayLimit:]...)
	}
	for subscriber := range hub.subscribers {
		if subscriber.tenant != tenantID {
			continue
		}
		select {
		case subscriber.events <- event:
		default:
			delete(hub.subscribers, subscriber)
			close(subscriber.events)
		}
	}
}

func (hub *eventHub) subscribe(actor, tenantID string, after uint64, resume bool) ([]serverEvent, <-chan serverEvent, func(), error) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	state := hub.tenants[tenantID]
	if state == nil {
		state = &tenantEventState{}
		hub.tenants[tenantID] = state
	}
	if resume && (after > state.next || len(state.replay) > 0 && after < state.replay[0].ID-1) {
		return nil, nil, nil, errEventReplayExpired
	}
	actorCount, tenantCount := 0, 0
	for subscriber := range hub.subscribers {
		if subscriber.actor == actor {
			actorCount++
		}
		if subscriber.tenant == tenantID {
			tenantCount++
		}
	}
	if len(hub.subscribers) >= eventTotalConnections || actorCount >= eventActorConnections || tenantCount >= eventTenantConnections {
		return nil, nil, nil, errEventConnectionLimit
	}
	replay := make([]serverEvent, 0)
	if resume {
		for _, event := range state.replay {
			if event.ID > after {
				replay = append(replay, event)
			}
		}
	}
	subscriber := &eventSubscriber{actor: actor, tenant: tenantID, events: make(chan serverEvent, 16)}
	hub.subscribers[subscriber] = struct{}{}
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			hub.mu.Lock()
			if _, ok := hub.subscribers[subscriber]; ok {
				delete(hub.subscribers, subscriber)
				close(subscriber.events)
			}
			hub.mu.Unlock()
		})
	}
	return replay, subscriber.events, cancel, nil
}

var errEventConnectionLimit = errors.New("event connection limit exceeded")

func (h *Handler) serverEvents(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeAudit(w, r) {
		return
	}
	if r.URL.RawQuery != "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_query", "events accepts no query parameters")
		return
	}
	if accept := r.Header.Get("Accept"); accept != "" && !strings.Contains(accept, "text/event-stream") {
		writeAPIError(w, http.StatusNotAcceptable, "event_stream_required", "Accept must include text/event-stream")
		return
	}
	after, resume := uint64(0), false
	if value := r.Header.Get("Last-Event-ID"); value != "" {
		resume = true
		var err error
		after, err = strconv.ParseUint(value, 10, 64)
		if err != nil || after == 0 {
			writeAPIError(w, http.StatusBadRequest, "invalid_last_event_id", "Last-Event-ID must be a positive decimal integer")
			return
		}
	}
	principal, _ := r.Context().Value(principalKey{}).(identity.Principal)
	tenantID, _ := tenant.FromContext(r.Context())
	h.startAlertWatcher()
	replay, events, cancel, err := h.events.subscribe(principal.Actor, tenantID, after, resume)
	if errors.Is(err, errEventReplayExpired) {
		writeAPIError(w, http.StatusConflict, "event_replay_expired", "event replay window expired; reconnect without Last-Event-ID and refresh")
		return
	}
	if errors.Is(err, errEventConnectionLimit) {
		w.Header().Set("Retry-After", "5")
		writeAPIError(w, http.StatusTooManyRequests, "event_connection_limit", "event connection budget exceeded")
		return
	}
	defer cancel()
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeAPIError(w, http.StatusServiceUnavailable, "event_stream_unavailable", "streaming is unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, "retry: 3000\n\n")
	for _, event := range replay {
		writeServerEvent(w, event)
	}
	flusher.Flush()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event, open := <-events:
			if !open {
				return
			}
			writeServerEvent(w, event)
			flusher.Flush()
		case <-heartbeat.C:
			_, _ = fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

func (h *Handler) startAlertWatcher() {
	if h.alerts == nil || h.eventContext == nil {
		return
	}
	h.eventOnce.Do(func() {
		tenants := append([]string(nil), h.auth.TenantIDs...)
		if len(tenants) == 0 {
			id := h.auth.DefaultTenant
			if id == "" {
				id = "local"
			}
			tenants = []string{id}
		}
		go watchAlerts(h.eventContext, h.alerts, tenants, h.events, 15*time.Second)
	})
}

func watchAlerts(ctx context.Context, backend AlertBackend, tenants []string, hub *eventHub, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var previous [32]byte
	havePrevious := false
	for {
		readContext, cancel := context.WithTimeout(ctx, 7*time.Second)
		snapshot, err := backend.AlertRules(readContext, time.Now().UTC())
		cancel()
		if err == nil {
			fingerprint := alertFingerprint(snapshot)
			if havePrevious && fingerprint != previous {
				for _, tenantID := range tenants {
					hub.publish(tenantID, "alerts")
				}
			}
			previous, havePrevious = fingerprint, true
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func alertFingerprint(snapshot prometheusbackend.AlertSnapshot) [32]byte {
	value := struct {
		Rules        []prometheusbackend.AlertRule `json:"rules"`
		MissingRules []string                      `json:"missingRules"`
		ConsoleURL   string                        `json:"consoleUrl"`
	}{snapshot.Rules, snapshot.MissingRules, snapshot.ConsoleURL}
	encoded, _ := json.Marshal(value)
	return sha256.Sum256(encoded)
}

func writeServerEvent(w http.ResponseWriter, event serverEvent) {
	payload, _ := json.Marshal(event)
	_, _ = fmt.Fprintf(w, "id: %d\nevent: invalidate\ndata: %s\n\n", event.ID, payload)
}
