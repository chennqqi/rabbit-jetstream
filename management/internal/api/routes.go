package api

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"

	adminui "github.com/chennqqi/rabbit-jetstream/admin-ui"
	contract "github.com/chennqqi/rabbit-jetstream/api"
	"github.com/chennqqi/rabbit-jetstream/management/internal/diagnostics"
)

func newHandler(client Backend, logger *slog.Logger, name, version string, monitor Monitor, control ControllerMonitor, auth AuthConfig, console ...ConsoleConfig) http.Handler {
	audit, _ := client.(AuditBackend)
	eventContext, eventCancel := context.WithCancel(context.Background())
	auth.OperatorTokens = cleanTokens(auth.OperatorTokens)
	auth.AuditorTokens = cleanTokens(auth.AuditorTokens)
	h := &Handler{client: client, monitor: monitor, logger: logger, name: name, version: version, started: time.Now(), auth: auth, controller: control, metrics: NewMetrics(), audit: audit, diagnostics: diagnostics.NewStore(diagnostics.Config{}), events: newEventHub(), eventContext: eventContext, eventCancel: eventCancel, loginLimiter: newLoginLimiter(), trustedProxyHops: auth.TrustedProxyHops}
	if len(console) > 0 {
		h.console = console[0]
		h.history = console[0].History
		h.alerts = console[0].Alerts
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /readyz", h.ready)
	mux.HandleFunc("GET /api/v1/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Header().Set("Cache-Control", "public, max-age=300")
		_, _ = w.Write(contract.OpenAPI)
	})
	mux.HandleFunc("GET /api/v1/native-sdk-contract.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=300")
		_, _ = w.Write(contract.NativeSDK)
	})
	mux.HandleFunc("GET /api/v1/info", h.info)
	mux.HandleFunc("POST /api/v1/auth/login", h.localLogin)
	mux.HandleFunc("GET /api/v1/session", h.session)
	mux.HandleFunc("GET /api/v1/access/tenants", h.accessTenants)
	mux.HandleFunc("GET /api/v1/access/accounts", h.accessAccounts)
	mux.HandleFunc("POST /api/v1/access/accounts", h.createAccessAccount)
	mux.HandleFunc("PUT /api/v1/access/accounts/{username}", h.updateAccessAccount)
	mux.HandleFunc("DELETE /api/v1/access/accounts/{username}", h.deleteAccessAccount)
	mux.HandleFunc("GET /api/v1/oidc/config", h.browserOIDCConfig)
	mux.HandleFunc("POST /api/v1/oidc/token", h.browserOIDCToken)
	mux.HandleFunc("GET /api/v1/console/capabilities", h.consoleCapabilities)
	mux.HandleFunc("GET /api/v1/console/build", h.consoleBuild)
	mux.HandleFunc("GET /api/v1/console/queue-schema", h.queueSchema)
	mux.HandleFunc("GET /api/v1/cluster", h.cluster)
	mux.HandleFunc("GET /api/v1/nodes", h.nodes)
	mux.HandleFunc("GET /api/v1/nodes/{node}/connections", h.nodeConnections)
	mux.HandleFunc("POST /api/v1/nodes/{node}/connections/search", h.nodeConnectionIdentitySearch)
	mux.HandleFunc("GET /api/v1/nodes/{node}/connections/{cid}", h.nodeConnection)
	mux.HandleFunc("GET /api/v1/nodes/{node}/connections/{cid}/subscriptions", h.nodeConnectionSubscriptions)
	mux.HandleFunc("GET /api/v1/streams", h.streams)
	mux.HandleFunc("GET /api/v1/streams/{stream}", h.stream)
	mux.HandleFunc("GET /api/v1/streams/{stream}/consumers", h.consumers)
	mux.HandleFunc("GET /api/v1/streams/{stream}/consumers/{consumer}", h.consumer)
	mux.HandleFunc("GET /api/v1/consumers", h.globalConsumers)
	mux.HandleFunc("POST /api/v1/consumers/refresh", h.refreshGlobalConsumers)
	mux.HandleFunc("PUT /api/v1/queues/{queue}", h.applyQueue)
	mux.HandleFunc("POST /api/v1/queues/{queue}/preview", h.previewQueue)
	mux.HandleFunc("POST /api/v1/queues/import-plan", h.planQueueImport)
	mux.HandleFunc("POST /api/v1/queues/change-plan", h.planQueueChanges)
	mux.HandleFunc("GET /api/v1/queues/{queue}/delete-preview", h.previewDeleteQueue)
	mux.HandleFunc("DELETE /api/v1/queues/{queue}", h.deleteQueue)
	mux.HandleFunc("GET /api/v1/queues", h.queues)
	mux.HandleFunc("GET /api/v1/queues/{queue}", h.queue)
	mux.HandleFunc("GET /api/v1/queues/{queue}/consumers", h.queueConsumers)
	mux.HandleFunc("GET /api/v1/queues/{queue}/routing-probe", h.queueRoutingProbe)
	mux.HandleFunc("GET /api/v1/queues/{queue}/export", h.exportQueue)
	mux.HandleFunc("GET /api/v1/controller", h.controllerStatus)
	mux.HandleFunc("GET /api/v1/audit", h.auditEvents)
	mux.HandleFunc("GET /api/v1/audit/requests/{requestID}", h.auditRequest)
	mux.HandleFunc("GET /api/v1/audit/windows", h.auditWindow)
	mux.HandleFunc("GET /api/v1/events", h.serverEvents)
	mux.HandleFunc("POST /api/v1/diagnostics/jobs", h.createDiagnosticJob)
	mux.HandleFunc("GET /api/v1/diagnostics/jobs/{id}", h.diagnosticJob)
	mux.HandleFunc("DELETE /api/v1/diagnostics/jobs/{id}", h.cancelDiagnosticJob)
	mux.HandleFunc("GET /api/v1/diagnostics/jobs/{id}/download", h.downloadDiagnosticJob)
	mux.HandleFunc("GET /api/v1/history", h.metricHistory)
	mux.HandleFunc("GET /api/v1/alerts", h.operationalAlerts)
	mux.HandleFunc("GET /metrics", h.prometheus)
	mux.Handle("GET /admin/", adminui.Handler())
	mux.HandleFunc("GET /admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusPermanentRedirect)
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/admin/", http.StatusTemporaryRedirect)
	})
	return &managedHandler{Handler: otelhttp.NewHandler(securityHeaders(h.logging(h.instrument(h.protectResourceReads(mux)))), "rabbit-jetstream.management.http"), diagnostics: h.diagnostics, cancel: eventCancel}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		if r.URL.Path != "/api/v1/openapi.yaml" && r.URL.Path != "/api/v1/native-sdk-contract.json" && !strings.HasPrefix(r.URL.Path, "/admin/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		attributes := []any{"method", r.Method, "path", r.URL.Path, "duration", time.Since(started)}
		spanContext := trace.SpanContextFromContext(r.Context())
		if spanContext.IsValid() {
			attributes = append(attributes, "trace_id", spanContext.TraceID().String(), "span_id", spanContext.SpanID().String())
		}
		if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
			attributes = append(attributes, "request_id", requestID)
		}
		h.logger.Debug("http request", attributes...)
	})
}
