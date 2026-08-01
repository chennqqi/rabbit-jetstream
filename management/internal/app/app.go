package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/chennqqi/rabbit-jetstream/internal/redact"
	"github.com/chennqqi/rabbit-jetstream/management/internal/api"
	managementauth "github.com/chennqqi/rabbit-jetstream/management/internal/auth"
	"github.com/chennqqi/rabbit-jetstream/management/internal/config"
	"github.com/chennqqi/rabbit-jetstream/management/internal/controller"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
	"github.com/chennqqi/rabbit-jetstream/management/internal/monitoring"
	"github.com/chennqqi/rabbit-jetstream/management/internal/observability"
)

type App struct {
	cfg               config.Config
	logger            *slog.Logger
	client            *jetstream.Client
	server            *http.Server
	controller        *controller.Controller
	shutdownTelemetry func(context.Context) error
}

func New(cfg config.Config, logger *slog.Logger, version string) (*App, error) {
	telemetryCtx, telemetryCancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer telemetryCancel()
	shutdownTelemetry, err := observability.Init(telemetryCtx, observability.Config{Endpoint: cfg.OTLPTraceEndpoint, ServiceName: cfg.Name, ServiceVersion: version, SampleRatio: cfg.OTELSampleRatio, AllowInsecure: cfg.OTELAllowInsecure})
	if err != nil {
		return nil, err
	}
	client, err := jetstream.Connect(cfg)
	if err != nil {
		_ = shutdownTelemetry(context.Background())
		return nil, err
	}
	control := controller.New(client, logger, cfg.InstanceID, cfg.ControllerEnabled, cfg.ControllerInterval, cfg.ControllerLeaseTTL)
	var oidcVerifier *managementauth.OIDCVerifier
	if cfg.OIDCIssuer != "" {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
		defer cancel()
		oidcVerifier, err = managementauth.NewOIDC(ctx, managementauth.OIDCConfig{Issuer: cfg.OIDCIssuer, Audience: cfg.OIDCAudience, RoleClaim: cfg.OIDCRoleClaim, OperatorRole: cfg.OIDCOperatorRole, AuditorRole: cfg.OIDCAuditorRole, AllowInsecureIssuer: cfg.OIDCAllowInsecure})
		if err != nil {
			client.Close()
			_ = shutdownTelemetry(context.Background())
			return nil, err
		}
	}
	operatorTokens := cfg.AdminTokens
	if len(operatorTokens) == 0 && cfg.AdminToken != "" {
		operatorTokens = []string{cfg.AdminToken}
	}
	server := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: api.NewWithControllerAuth(client, logger, cfg.Name, version, monitoring.New(cfg.NATSMonitorURLs, cfg.ConnectTimeout), control, api.AuthConfig{
			OperatorTokens: operatorTokens,
			AuditorTokens:  cfg.AuditTokens,
			OIDC:           oidcVerifier,
		}),
		ReadHeaderTimeout: cfg.ConnectTimeout,
	}
	return &App{cfg: cfg, logger: logger, client: client, server: server, controller: control, shutdownTelemetry: shutdownTelemetry}, nil
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
