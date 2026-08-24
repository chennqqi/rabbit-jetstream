// Command jetstream-cleanup removes test-only streams with an explicit prefix.
// It defaults to dry-run and exists so native qualification never needs source
// code or an interactive NATS CLI on the remote host.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/chennqqi/rabbit-jetstream/internal/natsclient"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	server := flag.String("server", nats.DefaultURL, "NATS server URL")
	prefix := flag.String("prefix", "", "required exact stream-name prefix")
	execute := flag.Bool("execute", false, "delete matching streams; otherwise dry-run")
	timeout := flag.Duration("timeout", 30*time.Second, "operation timeout")
	flag.Parse()
	if *prefix == "" || *timeout <= 0 {
		return errors.New("non-empty --prefix and positive --timeout are required")
	}
	if err := validatePrefix(*prefix); err != nil {
		return err
	}
	connectionOptions, err := natsclient.Options(natsclient.FromEnv())
	if err != nil {
		return err
	}
	connectionOptions = append(connectionOptions, nats.Timeout(5*time.Second))
	nc, err := nats.Connect(*server, connectionOptions...)
	if err != nil {
		return fmt.Errorf("connect to NATS: %w", err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("open JetStream: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	matched := 0
	for name := range js.StreamNames(nats.Context(ctx)) {
		if !strings.HasPrefix(name, *prefix) {
			continue
		}
		matched++
		if !*execute {
			fmt.Println("would delete", name)
			continue
		}
		if err := js.DeleteStream(name, nats.Context(ctx)); err != nil {
			return fmt.Errorf("delete stream %s: %w", name, err)
		}
		fmt.Println("deleted", name)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	fmt.Printf("matched=%d execute=%t\n", matched, *execute)
	return nil
}

func validatePrefix(prefix string) error {
	if !strings.HasPrefix(prefix, "RJSQ_sdk_perf_") {
		return fmt.Errorf("refusing non-benchmark prefix %q", prefix)
	}
	return nil
}
