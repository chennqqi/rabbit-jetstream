package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chennqqi/rabbit-jetstream/management/internal/app"
	"github.com/chennqqi/rabbit-jetstream/management/internal/config"
)

var version = "dev"
var revision = ""
var buildClean = "false"

func main() {
	if versionRequested(os.Args[1:]) {
		fmt.Fprintln(os.Stdout, version)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := healthcheck(); err != nil {
			fatal(err)
		}
		return
	}
	cfg := config.FromEnv()
	flag.StringVar(&cfg.HTTPAddr, "http", cfg.HTTPAddr, "management HTTP listen address")
	flag.StringVar(&cfg.NATSURL, "nats", cfg.NATSURL, "NATS server URL(s), comma-separated")
	flag.StringVar(&cfg.Name, "name", cfg.Name, "management instance name")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: config.LogLevel(cfg.LogLevel)}))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	service, err := app.New(cfg, logger, version, revision, buildClean)
	if err != nil {
		fatal(err)
	}
	defer service.Close()
	if err := service.Run(ctx); err != nil {
		fatal(err)
	}
}

func versionRequested(args []string) bool {
	return len(args) == 1 && (args[0] == "version" || args[0] == "--version")
}

func healthcheck() error {
	url := os.Getenv("RJS_HEALTHCHECK_URL")
	if url == "" {
		url = "http://127.0.0.1:8223/healthz"
	}
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("healthcheck request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck returned %s", response.Status)
	}
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
