package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/chennqqi/rabbit-jetstream/management/internal/app"
	"github.com/chennqqi/rabbit-jetstream/management/internal/config"
)

var version = "dev"

func main() {
	cfg := config.FromEnv()
	flag.StringVar(&cfg.HTTPAddr, "http", cfg.HTTPAddr, "management HTTP listen address")
	flag.StringVar(&cfg.NATSURL, "nats", cfg.NATSURL, "NATS server URL(s), comma-separated")
	flag.StringVar(&cfg.Name, "name", cfg.Name, "management instance name")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: config.LogLevel(cfg.LogLevel)}))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	service, err := app.New(cfg, logger, version)
	if err != nil {
		fatal(err)
	}
	defer service.Close()
	if err := service.Run(ctx); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
