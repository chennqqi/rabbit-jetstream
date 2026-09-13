package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"

	adminui "github.com/chennqqi/rabbit-jetstream/admin-ui"
	"github.com/chennqqi/rabbit-jetstream/internal/redact"
	"github.com/chennqqi/rabbit-jetstream/management/internal/api"
	managementauth "github.com/chennqqi/rabbit-jetstream/management/internal/auth"
	"github.com/chennqqi/rabbit-jetstream/management/internal/config"
	"github.com/chennqqi/rabbit-jetstream/management/internal/controller"
	"github.com/chennqqi/rabbit-jetstream/management/internal/identity"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
	"github.com/chennqqi/rabbit-jetstream/management/internal/monitoring"
	"github.com/chennqqi/rabbit-jetstream/management/internal/observability"
	prometheusbackend "github.com/chennqqi/rabbit-jetstream/management/internal/prometheus"
	"github.com/chennqqi/rabbit-jetstream/management/internal/qualification"
	"github.com/chennqqi/rabbit-jetstream/management/internal/tenant"
)

type App struct {
	cfg               config.Config
	logger            *slog.Logger
	client            appClient
	server            httpServer
	controller        controllerRunner
	shutdownTelemetry func(context.Context) error
	closeAPI          interface{ Close() }
}

type appClient interface {
	Close()
	ServerURL() string
}

type httpServer interface {
	ListenAndServe() error
	Shutdown(context.Context) error
}

type controllerRunner interface {
	Run(context.Context)
}

type controllerRuntime interface {
	controllerRunner
	api.ControllerMonitor
}

func New(cfg config.Config, logger *slog.Logger, version string, revision ...string) (*App, error) {
	if err := cfg.ValidateDeploymentProfile(); err != nil {
		return nil, err
	}
	if err := cfg.ValidateHTTPAccess(); err != nil {
		return nil, err
	}
	if err := cfg.ValidateLocalAuth(); err != nil {
		return nil, err
	}
	if err := cfg.ValidateTenancy(); err != nil {
		return nil, err
	}
	runtimeRevision := ""
	runtimeClean := false
	if len(revision) >= 1 {
		runtimeRevision = revision[0]
	}
	if len(revision) >= 2 {
		runtimeClean = revision[1] == "true"
	}
	qualificationReport, err := qualification.Load(cfg.ReleaseManifest, qualification.RuntimeIdentity{Version: version, Revision: runtimeRevision, Clean: runtimeClean, UIAssets: adminui.EmbeddedAssetIdentity()})
	if err != nil {
		return nil, fmt.Errorf("validate release manifest: %w", err)
	}
	telemetryCtx, telemetryCancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer telemetryCancel()
	shutdownTelemetry, err := observability.Init(telemetryCtx, observability.Config{Endpoint: cfg.OTLPTraceEndpoint, MetricsEndpoint: cfg.OTLPMetricEndpoint, MetricInterval: cfg.OTELMetricInterval, ServiceName: cfg.Name, ServiceVersion: version, SampleRatio: cfg.OTELSampleRatio, AllowInsecure: cfg.OTELAllowInsecure})
	if err != nil {
		return nil, err
	}
	definitions, err := tenant.Load(cfg.TenantsFile)
	if err != nil {
		_ = shutdownTelemetry(context.Background())
		return nil, fmt.Errorf("configure tenants: %w", err)
	}
	var backend api.Backend
	var monitor api.Monitor
	var control controllerRuntime
	var client appClient
	defaultTenant := "local"
	tenantIDs := []string{"local"}
	if len(definitions) == 0 {
		connected, connectErr := jetstream.Connect(cfg)
		if connectErr != nil {
			_ = shutdownTelemetry(context.Background())
			return nil, connectErr
		}
		backend, client = connected, connected
		monitor = monitoring.New(cfg.NATSMonitorURLs, cfg.ConnectTimeout)
		control = controller.New(connected, logger, cfg.InstanceID, cfg.ControllerEnabled, cfg.ControllerInterval, cfg.ControllerLeaseTTL)
	} else {
		sort.Slice(definitions, func(i, j int) bool { return definitions[i].ID < definitions[j].ID })
		router := &tenantRouter{clients: make(map[string]*jetstream.Client), monitors: make(map[string]*monitoring.Client), controllers: make(map[string]*controller.Controller), defaultID: definitions[0].ID}
		for _, definition := range definitions {
			tenantConfig := definition.Apply(cfg)
			connected, connectErr := jetstream.Connect(tenantConfig)
			if connectErr != nil {
				router.Close()
				_ = shutdownTelemetry(context.Background())
				return nil, fmt.Errorf("connect tenant %q: %w", definition.ID, connectErr)
			}
			router.clients[definition.ID] = connected
			router.monitors[definition.ID] = monitoring.New(tenantConfig.NATSMonitorURLs, tenantConfig.ConnectTimeout)
			router.controllers[definition.ID] = controller.New(connected, logger.With("tenant", definition.ID), tenantConfig.InstanceID, tenantConfig.ControllerEnabled, tenantConfig.ControllerInterval, tenantConfig.ControllerLeaseTTL)
		}
		backend, monitor, control, client = router, router, router, router
		defaultTenant = router.defaultID
		tenantIDs = make([]string, 0, len(definitions))
		for _, definition := range definitions {
			tenantIDs = append(tenantIDs, definition.ID)
		}
	}
	var localAuthenticator *managementauth.LocalAuthenticator
	if cfg.LocalAccountsFile != "" {
		localAuthenticator, err = managementauth.NewLocalFromFile(cfg.LocalAccountsFile, []byte(cfg.LocalAuthSigningKey), cfg.LocalAuthTTL)
		if err != nil {
			client.Close()
			_ = shutdownTelemetry(context.Background())
			return nil, fmt.Errorf("configure local authentication: %w", err)
		}
		known := map[string]struct{}{"local": {}}
		if len(definitions) > 0 {
			known = make(map[string]struct{}, len(definitions))
			for _, definition := range definitions {
				known[definition.ID] = struct{}{}
			}
		}
		if err = localAuthenticator.ValidateTenants(known); err != nil {
			client.Close()
			_ = shutdownTelemetry(context.Background())
			return nil, fmt.Errorf("configure local authentication: %w", err)
		}
	}
	var oidcVerifier identity.Verifier
	var browserOIDC api.BrowserOIDC
	if cfg.OIDCIssuer != "" {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
		defer cancel()
		var verifier *managementauth.OIDCVerifier
		verifier, err = managementauth.NewOIDC(ctx, managementauth.OIDCConfig{Issuer: cfg.OIDCIssuer, Audience: cfg.OIDCAudience, RoleClaim: cfg.OIDCRoleClaim, OperatorRole: cfg.OIDCOperatorRole, AuditorRole: cfg.OIDCAuditorRole, AllowInsecureIssuer: cfg.OIDCAllowInsecure, BrowserClientID: cfg.OIDCBrowserClientID, BrowserRedirectOrigin: cfg.OIDCBrowserRedirectOrigin})
		if err != nil {
			client.Close()
			_ = shutdownTelemetry(context.Background())
			return nil, err
		}
		oidcVerifier = verifier
		browserOIDC = verifier
	}
	operatorTokens := cfg.AdminTokens
	if len(operatorTokens) == 0 && cfg.AdminToken != "" {
		operatorTokens = []string{cfg.AdminToken}
	}
	var consoleQualification *api.QualificationReport
	if qualificationReport != nil {
		consoleQualification = &api.QualificationReport{Statement: qualificationReport.Statement, ManifestDigest: qualificationReport.ManifestDigest}
	}
	var history *prometheusbackend.Client
	if cfg.PrometheusURL != "" {
		history, err = prometheusbackend.New(prometheusbackend.Config{BaseURL: cfg.PrometheusURL, PublicURL: cfg.PrometheusPublicURL, BearerToken: cfg.PrometheusToken, AllowInsecure: cfg.PrometheusInsecure})
		if err != nil {
			client.Close()
			_ = shutdownTelemetry(context.Background())
			return nil, fmt.Errorf("configure Prometheus history: %w", err)
		}
	}
	handler := api.NewWithControllerAuth(backend, logger, cfg.Name, version, monitor, control, api.AuthConfig{
		OperatorTokens:   operatorTokens,
		AuditorTokens:    cfg.AuditTokens,
		Local:            localAuthenticator,
		LocalVerifier:    localAuthenticator,
		OIDC:             oidcVerifier,
		BrowserOIDC:      browserOIDC,
		RequireReadAuth:  !cfg.LocalDemo,
		DefaultTenant:    defaultTenant,
		TenantIDs:        tenantIDs,
		LocalAccounts:    localAuthenticator,
		TrustedProxyHops: cfg.TrustedProxyHops,
	}, api.ConsoleConfig{DeploymentProfile: cfg.DeploymentProfile, RuntimeRevision: runtimeRevision, RuntimeClean: runtimeClean, Qualification: consoleQualification, History: history, Alerts: history})
	server := newHTTPServer(cfg.HTTPAddr, handler, cfg.ConnectTimeout)
	closer, _ := handler.(interface{ Close() })
	return &App{cfg: cfg, logger: logger, client: client, server: server, controller: control, shutdownTelemetry: shutdownTelemetry, closeAPI: closer}, nil
}

func newHTTPServer(address string, handler http.Handler, readHeaderTimeout time.Duration) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
}

func (a *App) Run(ctx context.Context) error {
	go a.controller.Run(ctx)
	errCh := make(chan error, 1)
	go func() {
		a.logger.Info("management service started", "name", a.cfg.Name, "http", a.cfg.HTTPAddr, "nats", redact.URL(a.client.ServerURL()))
		errCh <- a.server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve management API: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
		defer cancel()
		a.logger.Info("shutting down")
		return a.server.Shutdown(shutdownCtx)
	}
}

func (a *App) Close() {
	if a.closeAPI != nil {
		a.closeAPI.Close()
	}
	a.client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
	defer cancel()
	if err := a.shutdownTelemetry(ctx); err != nil {
		a.logger.Error("flush telemetry", "error", err)
	}
}
