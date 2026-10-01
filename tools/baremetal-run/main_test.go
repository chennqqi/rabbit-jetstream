package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestHealthFailures(t *testing.T) {
	for _, fault := range []string{"", "transport", "status", "json", "identity", "restart"} {
		t.Run(fault, func(t *testing.T) {
			s := &supervisor{opts: options{port: 24220}, ids: map[int]string{}}
			s.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if fault == "transport" {
					return nil, errors.New("offline")
				}
				code := 200
				body := `{"server_id":"one","start":"today"}`
				switch fault {
				case "status":
					code = 503
				case "json":
					body = "{"
				case "identity":
					body = `{}`
				}
				return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			if fault == "restart" {
				s.ids[0] = "previous"
			}
			err := s.nodesHealthy()
			if fault == "" && err != nil {
				t.Fatal(err)
			}
			if fault != "" && err == nil {
				t.Fatal("bad health accepted")
			}
		})
	}
}

func TestSupervisorErrors(t *testing.T) {
	s := &supervisor{opts: options{bundle: t.TempDir(), output: t.TempDir()}}
	if s.checkPermanent() != nil {
		t.Fatal("empty permanent set")
	}
	done := make(chan struct{})
	close(done)
	c := &child{done: done, cmd: &exec.Cmd{Process: &os.Process{Pid: 12345}}}
	s.children = []*child{c}
	s.permanent = []*child{c}
	if s.checkPermanent() == nil {
		t.Fatal("exit not detected")
	}
	s.stop()
	if s.sync(context.Background(), "missing", "missing") == nil {
		t.Fatal("missing helper accepted")
	}
	if _, err := s.start("missing", "missing"); err == nil {
		t.Fatal("existing log replaced")
	}
	if save(filepath.Join(t.TempDir(), "bad"), make(chan int)) == nil {
		t.Fatal("invalid JSON accepted")
	}
	if checkReport(filepath.Join(t.TempDir(), "absent"), time.Minute) == nil {
		t.Fatal("missing report accepted")
	}
	path := filepath.Join(t.TempDir(), "invalid.json")
	os.WriteFile(path, []byte("{"), 0600)
	if checkReport(path, time.Minute) == nil {
		t.Fatal("invalid JSON accepted")
	}
}

func TestDiskGuard(t *testing.T) {
	for _, raw := range []string{"Avail\n0\n", "Avail\ninvalid\n", "unexpected", "Avail\n32212254719\n"} {
		if checkFreeDisk([]byte(raw)) == nil {
			t.Fatal("unsafe disk reading accepted")
		}
	}
	if err := checkFreeDisk([]byte("Avail\n32212254720\n")); err != nil {
		t.Fatal(err)
	}
}

func TestHostGuards(t *testing.T) {
	for _, fault := range []string{"", "read", "memory", "invalid-memory", "disk-command", "disk-space"} {
		t.Run(fault, func(t *testing.T) {
			read := func(path string) ([]byte, error) {
				if path != "/proc/meminfo" {
					t.Fatal(path)
				}
				switch fault {
				case "read":
					return nil, errors.New("read failed")
				case "memory":
					return []byte("MemAvailable: 1024 kB"), nil
				case "invalid-memory":
					return []byte("MemAvailable: invalid kB"), nil
				}
				return []byte("MemTotal: 64000000 kB\nMemAvailable: 32000000 kB"), nil
			}
			disk := func(path string) ([]byte, error) {
				if path != "private-state" {
					t.Fatal(path)
				}
				switch fault {
				case "disk-command":
					return nil, errors.New("df failed")
				case "disk-space":
					return []byte("Avail\n1"), nil
				}
				return []byte("Avail\n64000000000"), nil
			}
			err := checkHostWith("private-state", read, disk)
			if fault == "" && err != nil {
				t.Fatal(err)
			}
			if fault != "" && err == nil {
				t.Fatal("unsafe host accepted")
			}
		})
	}
}

func TestClusterReadiness(t *testing.T) {
	good := `{"meta_cluster":{"leader":"rjs-bare-1","cluster_size":3,"replicas":[{"name":"rjs-bare-2","current":true},{"name":"rjs-bare-3","current":true}]}}`
	for _, fault := range []string{"", "missing", "size", "leader", "offline", "lag", "current", "duplicate", "peer-count", "transport"} {
		t.Run(fault, func(t *testing.T) {
			body := good
			switch fault {
			case "missing":
				body = `{}`
			case "size":
				body = strings.ReplaceAll(good, `"cluster_size":3`, `"cluster_size":2`)
			case "leader":
				body = strings.ReplaceAll(good, `rjs-bare-1`, `unknown`)
			case "offline":
				body = strings.ReplaceAll(good, `"current":true`, `"current":true,"offline":true`)
			case "lag":
				body = strings.ReplaceAll(good, `"current":true`, `"current":true,"lag":1`)
			case "current":
				body = strings.ReplaceAll(good, `true`, `false`)
			case "duplicate":
				body = strings.ReplaceAll(good, `rjs-bare-3`, `rjs-bare-2`)
			case "peer-count":
				body = strings.ReplaceAll(good, `,{"name":"rjs-bare-3","current":true}`, ``)
			}
			s := &supervisor{opts: options{port: 24220}, client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if fault == "transport" {
					return nil, errors.New("offline")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}}
			err := s.clusterReady()
			if fault == "" && err != nil {
				t.Fatal(err)
			}
			if fault != "" && err == nil {
				t.Fatal("cluster not ready but accepted")
			}
		})
	}
}

func TestValidate(t *testing.T) {
	good := options{bundle: t.TempDir(), output: t.TempDir(), revision: strings.Repeat("a", 40), duration: 24 * time.Hour, port: 24220}
	if err := validate(good); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*options){func(o *options) { o.bundle = "relative" }, func(o *options) { o.revision = "invalid" }, func(o *options) { o.port = 1023 }, func(o *options) { o.port = 65526 }, func(o *options) { o.duration = time.Hour }, func(o *options) { o.calibrate = true }, func(o *options) { o.duration = 26 * time.Hour }} {
		o := good
		change(&o)
		if validate(o) == nil {
			t.Fatalf("unsafe options accepted: %+v", o)
		}
	}
	good.calibrate = true
	good.duration = 2 * time.Minute
	if err := validate(good); err != nil {
		t.Fatal(err)
	}
}

func TestExclusiveEvidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evidence.json")
	if err := save(path, "original"); err != nil {
		t.Fatal(err)
	}
	if save(path, "replacement") == nil {
		t.Fatal("overwrote existing file")
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "original") {
		t.Fatal("original changed")
	}
}

func TestOccupiedPortUntouched(t *testing.T) {
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	base := l.Addr().(*net.TCPAddr).Port
	if availablePorts(base) == nil {
		t.Fatal("accepted occupied port")
	}
	c, err := net.DialTimeout("tcp4", l.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("existing listener disrupted: %v", err)
	}
	c.Close()
}

func TestNodeConfigIsolation(t *testing.T) {
	for i := 0; i < 3; i++ {
		cfg := nodeConfig(t.TempDir(), 24220, i, "secret")
		if cfg["host"] != "127.0.0.1" || cfg["port"] != 24220+i {
			t.Fatal(cfg)
		}
		cluster := cfg["cluster"].(map[string]any)
		if cluster["host"] != "127.0.0.1" || cluster["port"] != 24223+i {
			t.Fatal(cluster)
		}
		if len(cluster["routes"].([]string)) != 2 {
			t.Fatal(cluster)
		}
		js := cfg["jetstream"].(map[string]any)
		if js["max_file_store"] != 2<<30 || js["max_memory_store"] != 128<<20 {
			t.Fatal(js)
		}
		if cfg["authorization"].(map[string]any)["password"] != "secret" {
			t.Fatal("missing auth")
		}
	}
}

func TestReportGates(t *testing.T) {
	good := map[string]any{"requested_messages": 600000, "published": 600000, "consumed": 600000, "replicas": 3, "publish_messages_per_second": 5000, "consume_messages_per_second": 4999, "publish_latency_p99_millis": 1, "duration_seconds": 120}
	check := func(v map[string]any) error {
		raw, _ := json.Marshal(v)
		path := filepath.Join(t.TempDir(), "report.json")
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		return checkReport(path, 2*time.Minute)
	}
	if err := check(good); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]any{"requested_messages": 0, "consumed": 599999, "published": 599999, "missing": 1, "duplicates": 1, "corrupt": 1, "publish_retries": 1, "consume_retries": 1, "replicas": 1, "publish_messages_per_second": 4899, "consume_messages_per_second": 4899, "publish_latency_p99_millis": 11, "duration_seconds": 119} {
		t.Run(key, func(t *testing.T) {
			v := map[string]any{}
			for k, x := range good {
				v[k] = x
			}
			v[key] = value
			if check(v) == nil {
				t.Fatal("bad report accepted")
			}
		})
	}
}
