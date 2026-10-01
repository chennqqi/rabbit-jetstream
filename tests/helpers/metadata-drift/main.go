// Test-only helper for the isolated candidate-live fixture. Never shipped.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/natsclient"
	"github.com/nats-io/nats.go"
	jsapi "github.com/nats-io/nats.go/jetstream"
)

const queue = "live_metadata_check"
const streamName = "RJSQ_" + queue
const owner = "rabbit-jetstream.io/queue"
const empty = "rabbit-jetstream.io/label.empty"
const removed = "rabbit-jetstream.io/label.removed"

func main() {
	endpoint := flag.String("nats", "", "owned loopback NATS URL")
	mode := flag.String("mode", "", "inject, verify, conflict or verify-conflict")
	flag.Parse()
	if err := run(*endpoint, *mode); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(endpoint, mode string) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "nats" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return fmt.Errorf("explicit loopback fixture URL required")
	}
	if mode != "inject" && mode != "verify" && mode != "conflict" && mode != "verify-conflict" {
		return fmt.Errorf("unknown mode")
	}
	options, err := natsclient.Options(natsclient.FromEnv())
	if err != nil {
		return err
	}
	options = append(options, nats.NoReconnect(), nats.Timeout(3*time.Second))
	conn, err := nats.Connect(endpoint, options...)
	if err != nil {
		return err
	}
	defer conn.Close()
	js, err := jsapi.New(conn)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	stream, err := js.Stream(ctx, streamName)
	if err != nil {
		return err
	}
	config := stream.CachedInfo().Config
	expectedOwner := queue
	if mode == "verify-conflict" {
		expectedOwner = "metadata_fixture_foreign"
	}
	if config.Metadata[owner] != expectedOwner {
		return fmt.Errorf("not the owned Stream fixture")
	}
	lister := stream.ListConsumers(ctx)
	consumers := []jsapi.ConsumerConfig{}
	for info := range lister.Info() {
		if info.Config.Metadata[owner] != expectedOwner {
			return fmt.Errorf("unexpected Consumer ownership")
		}
		consumers = append(consumers, info.Config)
	}
	if err := lister.Err(); err != nil {
		return err
	}
	if len(consumers) != 3 {
		return fmt.Errorf("expected exactly three fixture Consumers, got %d", len(consumers))
	}
	check := func(metadata map[string]string) error {
		if metadata["external/test-owner"] != "preserve" {
			return fmt.Errorf("external metadata missing")
		}
		if _, ok := metadata[removed]; ok {
			return fmt.Errorf("removed managed label retained")
		}
		if value, ok := metadata[empty]; !ok || value != "" {
			return fmt.Errorf("empty managed label missing")
		}
		return nil
	}
	inject := func(metadata map[string]string) {
		if mode == "conflict" {
			metadata[owner] = "metadata_fixture_foreign"
			return
		}
		metadata["external/test-owner"] = "preserve"
		metadata[removed] = ""
		delete(metadata, empty)
	}
	if mode == "inject" || mode == "conflict" {
		inject(config.Metadata)
		if _, err := js.CreateOrUpdateStream(ctx, config); err != nil {
			return err
		}
		for _, consumer := range consumers {
			inject(consumer.Metadata)
			if _, err := js.CreateOrUpdateConsumer(ctx, streamName, consumer); err != nil {
				return err
			}
		}
	} else {
		if err := check(config.Metadata); err != nil {
			return fmt.Errorf("Stream: %w", err)
		}
		for _, consumer := range consumers {
			if err := check(consumer.Metadata); err != nil {
				return fmt.Errorf("Consumer %s: %w", consumer.Name, err)
			}
		}
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"mode": mode, "queue": queue, "resources": len(consumers) + 1})
}
