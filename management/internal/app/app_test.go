package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chennqqi/rabbit-jetstream/management/internal/config"
)

type fakeClient struct {
	closed bool
	url    string
}

func (c *fakeClient) Close()            { c.closed = true }
func (c *fakeClient) ServerURL() string { return c.url }

type fakeController struct {
	started chan struct{}
}

func (c *fakeController) Run(ctx context.Context) {
	close(c.started)
	<-ctx.Done()
}

type fakeServer struct {
	listenErr   error
	shutdownErr error
	started     chan struct{}
	stopped     chan struct{}
	once        sync.Once
}

func (s *fakeServer) ListenAndServe() error {
	close(s.started)
	if s.listenErr != nil {
		return s.listenErr
	}
	<-s.stopped
	return http.ErrServerClosed
}

func (s *fakeServer) Shutdown(context.Context) error {
	s.once.Do(func() { close(s.stopped) })
	return s.shutdownErr
}

func testApp(server httpServer, client appClient, control controllerRunner, logger *slog.Logger) *App {
	return &App{
		cfg:               config.Config{Name: "test", HTTPAddr: "127.0.0.1:0", ShutdownTimeout: time.Second},
		logger:            logger,
		client:            client,
		server:            server,
		controller:        control,
		shutdownTelemetry: func(context.Context) error { return nil },
	}
}

func TestHTTPServerHasProductionResourceLimits(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	server := newHTTPServer(":8223", handler, 5*time.Second)
	if server.Addr != ":8223" || server.Handler == nil {
		t.Fatal("server address or handler was not retained")
	}
	if server.ReadHeaderTimeout != 5*time.Second || server.ReadTimeout != 15*time.Second || server.WriteTimeout != 30*time.Second || server.IdleTimeout != 60*time.Second {
		t.Fatalf("unexpected HTTP timeouts: header=%s read=%s write=%s idle=%s", server.ReadHeaderTimeout, server.ReadTimeout, server.WriteTimeout, server.IdleTimeout)
	}
	if server.MaxHeaderBytes != 64<<10 {
		t.Fatalf("MaxHeaderBytes=%d, want %d", server.MaxHeaderBytes, 64<<10)
	}
}

func TestRunReturnsListenError(t *testing.T) {
	want := errors.New("listen failed")
	server := &fakeServer{listenErr: want, started: make(chan struct{}), stopped: make(chan struct{})}
	control := &fakeController{started: make(chan struct{})}
	var logs bytes.Buffer
	application := testApp(server, &fakeClient{url: "nats://user:secret@nats:4222"}, control, slog.New(slog.NewJSONHandler(&logs, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := application.Run(ctx)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "serve management API") {
		t.Fatalf("Run() error = %v, want wrapped listen error", err)
	}
	select {
	case <-control.started:
	case <-time.After(time.Second):
		t.Fatal("controller was not started")
	}
	if strings.Contains(logs.String(), "user") || strings.Contains(logs.String(), "secret") || !strings.Contains(logs.String(), "nats://nats:4222") {
		t.Fatalf("startup log did not redact NATS credentials: %s", logs.String())
	}
}

func TestRunTreatsServerClosedAsClean(t *testing.T) {
	server := &fakeServer{listenErr: http.ErrServerClosed, started: make(chan struct{}), stopped: make(chan struct{})}
	application := testApp(server, &fakeClient{url: "nats://nats:4222"}, &fakeController{started: make(chan struct{})}, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := application.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
}

func TestRunShutsDownOnCancellation(t *testing.T) {
	server := &fakeServer{started: make(chan struct{}), stopped: make(chan struct{})}
	control := &fakeController{started: make(chan struct{})}
	application := testApp(server, &fakeClient{url: "nats://nats:4222"}, control, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()
	<-server.started
	<-control.started
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	select {
	case <-server.stopped:
	default:
		t.Fatal("HTTP server was not shut down")
	}
}

func TestRunReturnsShutdownError(t *testing.T) {
	want := errors.New("shutdown failed")
	server := &fakeServer{shutdownErr: want, started: make(chan struct{}), stopped: make(chan struct{})}
	application := testApp(server, &fakeClient{url: "nats://nats:4222"}, &fakeController{started: make(chan struct{})}, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()
	<-server.started
	cancel()
	if err := <-done; !errors.Is(err, want) {
		t.Fatalf("Run() error = %v, want %v", err, want)
	}
}

func TestCloseClosesClientAndReportsTelemetryFailure(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	client := &fakeClient{url: "nats://nats:4222"}
	application := testApp(&fakeServer{}, client, &fakeController{}, logger)
	application.shutdownTelemetry = func(context.Context) error { return errors.New("flush failed") }

	application.Close()
	if !client.closed {
		t.Fatal("NATS client was not closed")
	}
	if !strings.Contains(logs.String(), "flush telemetry") || !strings.Contains(logs.String(), "flush failed") {
		t.Fatalf("telemetry failure was not logged: %s", logs.String())
	}
}
