package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
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

func New(cfg config.Config, logger *slog.Logger, version string, revision ...string) (*App, error) {
	if err := cfg.ValidateDeploymentProfile(); err != nil {
		return nil, err
	}
	if err := cfg.ValidateHTTPAccess(); err != nil {
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
	client, err := jetstream.Connect(cfg)
	if err != nil {
		_ = shutdownTelemetry(context.Background())
		return nil, err
	}
	control := controller.New(client, logger, cfg.InstanceID, cfg.ControllerEnabled, cfg.ControllerInterval, cfg.ControllerLeaseTTL)
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
	handler := api.NewWithControllerAuth(client, logger, cfg.Name, version, monitoring.New(cfg.NATSMonitorURLs, cfg.ConnectTimeout), control, api.AuthConfig{
		OperatorTokens:  operatorTokens,
		AuditorTokens:   cfg.AuditTokens,
		OIDC:            oidcVerifier,
		BrowserOIDC:     browserOIDC,
		RequireReadAuth: !cfg.LocalDemo,
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
