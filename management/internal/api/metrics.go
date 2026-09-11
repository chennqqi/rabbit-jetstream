package api

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type requestMetric struct {
	Method, Route string
	Status        int
	Count         uint64
	Duration      time.Duration
}

type Metrics struct {
	mu       sync.Mutex
	requests map[string]*requestMetric
}

func NewMetrics() *Metrics { return &Metrics{requests: make(map[string]*requestMetric)} }

func (m *Metrics) Observe(method, route string, status int, duration time.Duration) {
	if route == "" {
		route = "unmatched"
	}
	key := method + "\x00" + route + "\x00" + strconv.Itoa(status)
	m.mu.Lock()
	metric := m.requests[key]
	if metric == nil {
		metric = &requestMetric{Method: method, Route: route, Status: status}
		m.requests[key] = metric
	}
	metric.Count++
	metric.Duration += duration
	m.mu.Unlock()
}

func (m *Metrics) snapshot() []requestMetric {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]requestMetric, 0, len(m.requests))
	for _, metric := range m.requests {
		result = append(result, *metric)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Route != result[j].Route {
			return result[i].Route < result[j].Route
		}
		if result[i].Method != result[j].Method {
			return result[i].Method < result[j].Method
		}
		return result[i].Status < result[j].Status
	})
	return result
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (w *statusWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (h *Handler) instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		wrapped := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(wrapped, r)
		status := wrapped.status
		if status == 0 {
			status = http.StatusOK
		}
		h.metrics.Observe(r.Method, r.Pattern, status, time.Since(started))
	})
}

func (h *Handler) prometheus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var output strings.Builder
	metricHelp(&output, "rjs_build_info", "Build and service identity.", "gauge")
	metric(&output, "rjs_build_info", map[string]string{"name": h.name, "version": h.version}, 1)
	metricHelp(&output, "rjs_uptime_seconds", "Management service uptime in seconds.", "gauge")
	metric(&output, "rjs_uptime_seconds", nil, time.Since(h.started).Seconds())

	metricHelp(&output, "rjs_jetstream_up", "Whether the JetStream account is reachable.", "gauge")
	if account, err := h.client.AccountInfo(ctx); err == nil {
		metric(&output, "rjs_jetstream_up", nil, 1)
		metricHelp(&output, "rjs_jetstream_memory_bytes", "JetStream account memory in use.", "gauge")
		metric(&output, "rjs_jetstream_memory_bytes", nil, float64(account.MemoryUsed))
		metricHelp(&output, "rjs_jetstream_storage_bytes", "JetStream account storage in use.", "gauge")
		metric(&output, "rjs_jetstream_storage_bytes", nil, float64(account.StorageUsed))
		metricHelp(&output, "rjs_jetstream_streams", "Number of JetStream Streams.", "gauge")
		metric(&output, "rjs_jetstream_streams", nil, float64(account.Streams))
		metricHelp(&output, "rjs_jetstream_consumers", "Number of JetStream Consumers.", "gauge")
		metric(&output, "rjs_jetstream_consumers", nil, float64(account.Consumers))
		metricHelp(&output, "rjs_jetstream_api_errors_total", "JetStream API errors observed by the account.", "counter")
		metric(&output, "rjs_jetstream_api_errors_total", nil, float64(account.APIErrors))
	} else {
		metric(&output, "rjs_jetstream_up", nil, 0)
	}

	if declarations, err := h.client.ListDeclarations(ctx); err == nil {
		metricHelp(&output, "rjs_queues_declared", "Number of persisted Queue declarations.", "gauge")
		metric(&output, "rjs_queues_declared", nil, float64(len(declarations)))
	}
	if streams, err := h.client.ListStreams(ctx); err == nil {
		metricHelp(&output, "rjs_queue_messages", "Messages stored in an owned logical Queue.", "gauge")
		metricHelp(&output, "rjs_queue_bytes", "Bytes stored in an owned logical Queue.", "gauge")
		for _, stream := range streams {
			queue := stream.Metadata["rabbit-jetstream.io/queue"]
			if queue == "" {
				continue
			}
			labels := map[string]string{"queue": queue}
			metric(&output, "rjs_queue_messages", labels, float64(stream.Messages))
			metric(&output, "rjs_queue_bytes", labels, float64(stream.Bytes))
		}
	}
	if h.monitor != nil {
		nodes := h.monitor.Nodes(ctx)
		metricHelp(&output, "rjs_nats_nodes", "NATS monitoring endpoints by availability state.", "gauge")
		metric(&output, "rjs_nats_nodes", map[string]string{"status": "available"}, float64(nodes.Available))
		metric(&output, "rjs_nats_nodes", map[string]string{"status": "degraded"}, float64(nodes.Degraded))
		metric(&output, "rjs_nats_nodes", map[string]string{"status": "unavailable"}, float64(nodes.Unavailable))
	}
	if h.controller != nil {
		status := h.controllerStatusFor(ctx)
		instance := map[string]string{"instance": status.InstanceID}
		metricHelp(&output, "rjs_controller_leader", "Whether this management instance is controller leader.", "gauge")
		metric(&output, "rjs_controller_leader", instance, boolFloat(status.Leader))
		metricHelp(&output, "rjs_controller_blocked_queues", "Queue declarations blocked in the latest controller pass.", "gauge")
		metric(&output, "rjs_controller_blocked_queues", instance, float64(status.Blocked))
		metricHelp(&output, "rjs_dlq_processed_total", "DLQ advisory processing attempts across all Queues on this instance, including redeliveries.", "counter")
		metric(&output, "rjs_dlq_processed_total", instance, float64(status.DLQProcessed))
		metricHelp(&output, "rjs_dlq_ignored_total", "DLQ advisory attempts ignored as invalid JSON or without a matching declared source on this instance.", "counter")
		metric(&output, "rjs_dlq_ignored_total", instance, float64(status.DLQIgnored))
		metricHelp(&output, "rjs_dlq_moved_total", "DLQ move attempts completed on this instance, including already-absent source messages; not unique transfers.", "counter")
		metric(&output, "rjs_dlq_moved_total", instance, float64(status.DLQMoved))
		metricHelp(&output, "rjs_dlq_failed_total", "Dead-letter move attempts that failed on this instance.", "counter")
		metric(&output, "rjs_dlq_failed_total", instance, float64(status.DLQFailed))
		metricHelp(&output, "rjs_controller_last_success_timestamp_seconds", "Unix timestamp of the latest successful leader pass.", "gauge")
		lastSuccess := float64(0)
		if !status.LastSuccess.IsZero() {
			lastSuccess = float64(status.LastSuccess.Unix())
		}
		metric(&output, "rjs_controller_last_success_timestamp_seconds", instance, lastSuccess)
	}

	metricHelp(&output, "rjs_http_requests_total", "HTTP requests by method, route pattern, and status.", "counter")
	metricHelp(&output, "rjs_http_request_duration_seconds_sum", "Accumulated HTTP request duration by method, route pattern, and status.", "counter")
	metricHelp(&output, "rjs_http_request_duration_seconds_count", "HTTP request count by method and route pattern.", "counter")
	for _, request := range h.metrics.snapshot() {
		labels := map[string]string{"method": request.Method, "route": request.Route, "code": strconv.Itoa(request.Status)}
		metric(&output, "rjs_http_requests_total", labels, float64(request.Count))
		metric(&output, "rjs_http_request_duration_seconds_sum", labels, request.Duration.Seconds())
		metric(&output, "rjs_http_request_duration_seconds_count", labels, float64(request.Count))
	}
	_, _ = w.Write([]byte(output.String()))
}

func metricHelp(output *strings.Builder, name, help, kind string) {
	fmt.Fprintf(output, "# HELP %s %s\n# TYPE %s %s\n", name, help, name, kind)
}

func metric(output *strings.Builder, name string, labels map[string]string, value float64) {
	output.WriteString(name)
	if len(labels) > 0 {
		keys := make([]string, 0, len(labels))
		for key := range labels {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		output.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				output.WriteByte(',')
			}
			fmt.Fprintf(output, "%s=\"%s\"", key, metricEscape(labels[key]))
		}
		output.WriteByte('}')
	}
	fmt.Fprintf(output, " %g\n", value)
}

func metricEscape(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\n", "\\n")
	return strings.ReplaceAll(value, "\"", "\\\"")
}

func boolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
