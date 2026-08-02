package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/redact"
	"github.com/chennqqi/rabbit-jetstream/management/internal/api"
	managementauth "github.com/chennqqi/rabbit-jetstream/management/internal/auth"
	"github.com/chennqqi/rabbit-jetstream/management/internal/config"
	"github.com/chennqqi/rabbit-jetstream/management/internal/controller"
	"github.com/chennqqi/rabbit-jetstream/management/internal/identity"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
	"github.com/chennqqi/rabbit-jetstream/management/internal/monitoring"
	"github.com/chennqqi/rabbit-jetstream/management/internal/observability"
)

type App struct {
	cfg               config.Config
	logger            *slog.Logger
	client            appClient
	server            httpServer
	controller        controllerRunner
	shutdownTelemetry func(context.Context) error
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

func New(cfg config.Config, logger *slog.Logger, version string) (*App, error) {
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
	if cfg.OIDCIssuer != "" {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
		defer cancel()
		var verifier *managementauth.OIDCVerifier
		verifier, err = managementauth.NewOIDC(ctx, managementauth.OIDCConfig{Issuer: cfg.OIDCIssuer, Audience: cfg.OIDCAudience, RoleClaim: cfg.OIDCRoleClaim, OperatorRole: cfg.OIDCOperatorRole, AuditorRole: cfg.OIDCAuditorRole, AllowInsecureIssuer: cfg.OIDCAllowInsecure})
		if err != nil {
			client.Close()
			_ = shutdownTelemetry(context.Background())
			return nil, err
		}
		oidcVerifier = verifier
	}
	operatorTokens := cfg.AdminTokens
	if len(operatorTokens) == 0 && cfg.AdminToken != "" {
		operatorTokens = []string{cfg.AdminToken}
	}
	handler := api.NewWithControllerAuth(client, logger, cfg.Name, version, monitoring.New(cfg.NATSMonitorURLs, cfg.ConnectTimeout), control, api.AuthConfig{
		OperatorTokens: operatorTokens,
		AuditorTokens:  cfg.AuditTokens,
		OIDC:           oidcVerifier,
	})
	server := newHTTPServer(cfg.HTTPAddr, handler, cfg.ConnectTimeout)
	return &App{cfg: cfg, logger: logger, client: client, server: server, controller: control, shutdownTelemetry: shutdownTelemetry}, nil
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
	a.client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
	defer cancel()
	if err := a.shutdownTelemetry(ctx); err != nil {
		a.logger.Error("flush telemetry", "error", err)
	}
}
