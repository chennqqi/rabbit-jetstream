package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/chennqqi/rabbit-jetstream/management/internal/api"
	"github.com/chennqqi/rabbit-jetstream/management/internal/config"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

type App struct {
	cfg    config.Config
	logger *slog.Logger
	client *jetstream.Client
	server *http.Server
}

func New(cfg config.Config, logger *slog.Logger, version string) (*App, error) {
	client, err := jetstream.Connect(cfg)
	if err != nil {
		return nil, err
	}
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.New(client, logger, cfg.Name, version),
		ReadHeaderTimeout: cfg.ConnectTimeout,
	}
	return &App{cfg: cfg, logger: logger, client: client, server: server}, nil
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		a.logger.Info("management service started", "name", a.cfg.Name, "http", a.cfg.HTTPAddr, "nats", a.client.ServerURL())
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

func (a *App) Close() { a.client.Close() }
