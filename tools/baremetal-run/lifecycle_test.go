package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The fixture subprocesses are copies of this test executable, never real NATS
// or host services. This exercises child ownership and cleanup on all platforms.
func TestMain(m *testing.M) {
	if os.Getenv("RJS_BARE_TEST_HELPER") == "1" {
		helperMain()
		return
	}
	os.Exit(m.Run())
}
func helperMain() {
	name := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	args := map[string]string{}
	for i := 1; i+1 < len(os.Args); i += 2 {
		args[os.Args[i]] = os.Args[i+1]
	}
	switch name {
	case "nativequal":
		_ = save(args["-output"], map[string]any{"nats_binary_sha256": strings.Repeat("a", 64)})
	case "nats-server":
		raw, _ := os.ReadFile(args["-c"])
		var cfg map[string]any
		_ = json.Unmarshal(raw, &cfg)
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/varz" {
				fmt.Fprint(w, `{"server_id":"fixture","start":"2026-01-01"}`)
			} else {
				fmt.Fprint(w, `{}`)
			}
		})
		_ = http.ListenAndServe(cfg["http"].(string), mux)
	case "rjs-management":
		_ = http.ListenAndServe(os.Getenv("RJS_HTTP_ADDR"), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) }))
	case "resource-sampler":
		time.Sleep(6500 * time.Millisecond)
		_ = save(args["-output"], map[string]any{})
	case "jetstream-bench":
		time.Sleep(6 * time.Second)
		d, _ := time.ParseDuration(args["-duration"])
		_ = save(args["-output"], map[string]any{"requested_messages": 600000, "published": 600000, "consumed": 600000, "replicas": 3, "publish_messages_per_second": 5000, "consume_messages_per_second": 5000, "publish_latency_p99_millis": 1, "duration_seconds": d.Seconds()})
	case "resourceaudit":
		_ = save(args["-output"], map[string]any{})
	case "perfevidence":
		_ = save(args["-evidence"], map[string]any{})
	}
	os.Exit(0)
}
func TestSupervisedLifecycle(t *testing.T) {
	uid, guard := effectiveUID, hostGuard
	t.Cleanup(func() { effectiveUID = uid; hostGuard = guard })
	effectiveUID = func() int { return 1000 }
	hostGuard = func(string) error { return nil }
	t.Setenv("RJS_BAREMETAL_CONFINED", "systemd-v1")
	t.Setenv("RJS_BARE_TEST_HELPER", "1")
	for _, calibrate := range []bool{true, false} {
		t.Run(fmt.Sprint(calibrate), func(t *testing.T) {
			bundle := t.TempDir()
			bin := filepath.Join(bundle, "bin/linux-amd64")
			if err := os.MkdirAll(bin, 0700); err != nil {
				t.Fatal(err)
			}
			executable, _ := os.Executable()
			for _, name := range []string{"nativequal", "nats-server", "rjs-management", "resource-sampler", "jetstream-bench", "resourceaudit", "perfevidence"} {
				if runtime.GOOS == "windows" {
					name += ".exe"
				}
				in, err := os.Open(executable)
				if err != nil {
					t.Fatal(err)
				}
				out, err := os.OpenFile(filepath.Join(bin, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
				if err != nil {
					t.Fatal(err)
				}
				_, err = io.Copy(out, in)
				in.Close()
				out.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			base := 0
			for attempt := 0; attempt < 20; attempt++ {
				l, err := net.Listen("tcp4", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				port := l.Addr().(*net.TCPAddr).Port
				l.Close()
				if port <= 65525 && availablePorts(port) == nil {
					base = port
					break
				}
			}
			if base == 0 {
				t.Fatal("no free test ports")
			}
			output := t.TempDir()
			duration := 24 * time.Hour
			if calibrate {
				duration = time.Minute
			}
			o := options{bundle: bundle, output: output, revision: strings.Repeat("a", 40), duration: duration, port: base, calibrate: calibrate}
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			if err := run(ctx, o); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(output, "run/completion.json"))
			if err != nil || !strings.Contains(string(raw), `"completed"`) {
				t.Fatalf("completion: %s %v", raw, err)
			}
			if err := availablePorts(base); err != nil {
				t.Fatalf("test children leaked: %v", err)
			}
			if run(ctx, o) == nil {
				t.Fatal("reused state")
			}
		})
	}
}
func TestRunFailsClosed(t *testing.T) {
	uid, guard := effectiveUID, hostGuard
	t.Cleanup(func() { effectiveUID = uid; hostGuard = guard })
	o := options{bundle: t.TempDir(), output: t.TempDir(), revision: strings.Repeat("a", 40), duration: 24 * time.Hour, port: 24220}
	effectiveUID = func() int { return 0 }
	if run(context.Background(), o) == nil {
		t.Fatal("root accepted")
	}
	effectiveUID = func() int { return 1000 }
	t.Setenv("RJS_BAREMETAL_CONFINED", "")
	if run(context.Background(), o) == nil {
		t.Fatal("unconfined launch accepted")
	}
	t.Setenv("RJS_BAREMETAL_CONFINED", "systemd-v1")
	hostGuard = func(string) error { return errors.New("guard") }
	if run(context.Background(), o) == nil {
		t.Fatal("missing verified helper accepted")
	}
	raw, _ := os.ReadFile(filepath.Join(o.output, "run/completion.json"))
	if !strings.Contains(string(raw), `"failed"`) {
		t.Fatal("failure not recorded")
	}
}
