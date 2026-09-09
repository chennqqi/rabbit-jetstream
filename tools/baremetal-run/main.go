// Command baremetal-run supervises an isolated, bounded native qualification run.
// It never installs software, invokes a container engine, or signals unrelated PIDs.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type options struct {
	bundle, output, revision string
	duration                 time.Duration
	port                     int
	calibrate                bool
}
type child struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error
}
type supervisor struct {
	opts      options
	children  []*child
	permanent []*child
	client    *http.Client
	ids       map[int]string
	started   time.Time
	env       []string
}

// Environment boundaries are injectable for lifecycle tests; production always
// uses the actual UID and host resource readings.
var effectiveUID = os.Geteuid
var hostGuard = checkHost

func main() {
	o := options{}
	flag.StringVar(&o.bundle, "bundle", "", "read-only bare-metal bundle")
	flag.StringVar(&o.output, "output", "", "existing private output parent (must contain no run directory)")
	flag.StringVar(&o.revision, "source-revision", "", "frozen runtime source revision")
	flag.DurationVar(&o.duration, "duration", 24*time.Hour, "workload duration")
	flag.IntVar(&o.port, "port-base", 24220, "ten consecutive loopback ports")
	flag.BoolVar(&o.calibrate, "calibrate", false, "short calibration, never release soak evidence")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, o); err != nil {
		fmt.Fprintln(os.Stderr, "qualification stopped:", err)
		os.Exit(1)
	}
}

func validate(o options) error {
	if !filepath.IsAbs(o.bundle) || !filepath.IsAbs(o.output) || len(o.revision) != 40 || strings.Trim(o.revision, "0123456789abcdef") != "" || o.port < 1024 || o.port > 65525 {
		return errors.New("absolute paths, valid revision and unprivileged port range required")
	}
	if o.calibrate {
		if o.duration < time.Minute || o.duration > 10*time.Minute {
			return errors.New("calibration must last 1–10 minutes")
		}
	} else if o.duration < 24*time.Hour || o.duration > 25*time.Hour {
		return errors.New("soak must last 24–25 hours")
	}
	return nil
}

func run(ctx context.Context, o options) (err error) {
	if err = validate(o); err != nil {
		return err
	}
	if effectiveUID() <= 0 {
		return errors.New("run under a dedicated non-root systemd service")
	}
	// The supervisor fails closed unless the launcher supplied the confinement contract.
	if os.Getenv("RJS_BAREMETAL_CONFINED") != "systemd-v1" {
		return errors.New("bounded systemd launcher is required")
	}
	if _, err = os.Stat(o.output); err != nil {
		return err
	}
	o.output = filepath.Join(o.output, "run")
	if err = os.Mkdir(o.output, 0700); err != nil {
		return fmt.Errorf("exclusive run directory: %w", err)
	}
	s := &supervisor{opts: o, client: &http.Client{Timeout: 2 * time.Second}, ids: map[int]string{}, started: time.Now().UTC()}
	defer func() {
		s.stop()
		status := "completed"
		message := ""
		if err != nil {
			status = "failed"
			message = err.Error()
		}
		_ = save(filepath.Join(o.output, "completion.json"), map[string]any{"status": status, "error": message, "finished_at": time.Now().UTC(), "calibration": o.calibrate, "source_revision": o.revision})
	}()
	if err = s.sync(ctx, "preflight", "nativequal", "-mode", "bare-metal", "-bundle", o.bundle, "-source-revision", o.revision, "-data-path", o.output, "-output", filepath.Join(o.output, "preflight.json")); err != nil {
		return err
	}
	if err = availablePorts(o.port); err != nil {
		return err
	}
	if err = hostGuard(o.output); err != nil {
		return err
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return err
	}
	password := hex.EncodeToString(secret)
	s.env = []string{"RJS_NATS_USER=qualification", "RJS_NATS_PASSWORD=" + password, "GOMAXPROCS=2"}
	urls, monitors := []string{}, []string{}
	for i := 0; i < 3; i++ {
		urls = append(urls, fmt.Sprintf("nats://127.0.0.1:%d", o.port+i))
		monitors = append(monitors, fmt.Sprintf("http://127.0.0.1:%d", o.port+6+i))
	}
	for i := 0; i < 3; i++ {
		cfg := nodeConfig(o.output, o.port, i, password)
		path := filepath.Join(o.output, fmt.Sprintf("nats-%d.json", i+1))
		if err = save(path, cfg); err != nil {
			return err
		}
		c, e := s.start(fmt.Sprintf("nats-%d", i+1), "nats-server", "-c", path)
		if e != nil {
			return e
		}
		s.permanent = append(s.permanent, c)
	}
	for deadline := time.Now().Add(60 * time.Second); ; {
		if err = s.nodesHealthy(); err == nil {
			break
		}
		if time.Now().After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	s.env = append(s.env, "RJS_HTTP_ADDR=127.0.0.1:"+strconv.Itoa(o.port+9), "RJS_NATS_URL="+strings.Join(urls, ","), "RJS_NATS_MONITOR_URLS="+strings.Join(monitors, ","), "RJS_METADATA_REPLICAS=3", "RJS_ADMIN_TOKEN="+password)
	c, e := s.start("management", "rjs-management")
	if e != nil {
		return e
	}
	s.permanent = append(s.permanent, c)
	readyURL := fmt.Sprintf("http://127.0.0.1:%d/readyz", o.port+9)
	for deadline := time.Now().Add(60 * time.Second); ; {
		if err = s.get(readyURL, nil); err == nil {
			break
		}
		if time.Now().After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	interval := 10 * time.Second
	nodes := []string{}
	for i, url := range monitors {
		nodes = append(nodes, fmt.Sprintf("nats-%d=%s", i+1, url))
	}
	// Start sampling immediately before the workload and continue slightly beyond it.
	sam, e := s.start("sampler", "resource-sampler", "-nodes", strings.Join(nodes, ","), "-output", filepath.Join(o.output, "resources.ndjson"), "-duration", (o.duration + 20*time.Second).String(), "-interval", interval.String(), "-disk-path", o.output)
	if e != nil {
		return e
	}
	args := []string{"-server", strings.Join(urls, ","), "-output", filepath.Join(o.output, "report.json"), "-messages", "0", "-duration", o.duration.String(), "-publish-rate", "5000", "-payload-bytes", "1024", "-publishers", "8", "-batch", "256", "-replicas", "3", "-publish-retry-timeout", "2m", "-consume-retry-timeout", "2m", "-timeout", "5m"}
	if err = save(filepath.Join(o.output, "command.json"), append([]string{s.bin("jetstream-bench")}, args...)); err != nil {
		return err
	}
	bench, e := s.start("workload", "jetstream-bench", args...)
	if e != nil {
		return e
	}
	if err = save(filepath.Join(o.output, "started.json"), map[string]any{"started_at": time.Now().UTC(), "expected_finish": time.Now().UTC().Add(o.duration), "duration_seconds": o.duration.Seconds(), "calibration": o.calibrate, "source_revision": o.revision, "supervisor_pid": os.Getpid(), "port_base": o.port, "target_rate": 5000, "max_file_store_per_node_bytes": 2 << 30, "min_host_available_memory_bytes": 16 << 30, "min_host_free_disk_bytes": 30 << 30}); err != nil {
		return err
	}
	guard := time.NewTicker(5 * time.Second)
	defer guard.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-bench.done:
			if bench.err != nil {
				return fmt.Errorf("workload failed: %w", bench.err)
			}
			goto finished
		case <-guard.C:
			if err = s.checkPermanent(); err != nil {
				return err
			}
			if err = hostGuard(o.output); err != nil {
				return err
			}
			if err = s.nodesHealthy(); err != nil {
				return err
			}
			if err = s.get(readyURL, nil); err != nil {
				return fmt.Errorf("management readiness: %w", err)
			}
			select {
			case <-sam.done:
				return errors.New("resource sampler exited before workload")
			default:
			}
		}
	}
finished:
	select {
	case <-sam.done:
		if sam.err != nil {
			return sam.err
		}
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(40 * time.Second):
		return errors.New("sampler completion timeout")
	}
	if err = s.checkPermanent(); err != nil {
		return err
	}
	if err = s.sync(ctx, "resource-audit", "resourceaudit", "-input", filepath.Join(o.output, "resources.ndjson"), "-output", filepath.Join(o.output, "resource-summary.json")); err != nil {
		return err
	}
	if err = checkReport(filepath.Join(o.output, "report.json"), o.duration); err != nil {
		return err
	}
	if !o.calibrate {
		var pre struct {
			NATSBinarySHA256 string `json:"nats_binary_sha256"`
		}
		raw, e := os.ReadFile(filepath.Join(o.output, "preflight.json"))
		if e != nil {
			return e
		}
		if e = json.Unmarshal(raw, &pre); e != nil {
			return e
		}
		if err = s.sync(ctx, "verify-soak", "perfevidence", "-create-inaugural", "-evidence", filepath.Join(o.output, "soak-evidence.json"), "-report", filepath.Join(o.output, "report.json"), "-resources", filepath.Join(o.output, "resources.ndjson"), "-native-preflight", filepath.Join(o.output, "preflight.json"), "-nats-binary-sha256", pre.NATSBinarySHA256, "-command-file", filepath.Join(o.output, "command.json"), "-sample-interval", "10", "-min-publish", "4900", "-min-consume", "4900", "-max-p99", "10"); err != nil {
			return err
		}
	}
	return nil
}

func nodeConfig(output string, base, index int, password string) map[string]any {
	routes := []string{}
	for i := 0; i < 3; i++ {
		if i != index {
			routes = append(routes, fmt.Sprintf("nats://cluster:%s@127.0.0.1:%d", password, base+3+i))
		}
	}
	return map[string]any{"server_name": fmt.Sprintf("rjs-bare-%d", index+1), "host": "127.0.0.1", "port": base + index, "http": "127.0.0.1:" + strconv.Itoa(base+6+index), "max_connections": 64,
		"authorization": map[string]any{"user": "qualification", "password": password}, "jetstream": map[string]any{"store_dir": filepath.Join(output, fmt.Sprintf("data-%d", index+1)), "max_memory_store": 128 << 20, "max_file_store": 2 << 30},
		"cluster": map[string]any{"name": "rjs-baremetal-qualification", "host": "127.0.0.1", "port": base + 3 + index, "authorization": map[string]any{"user": "cluster", "password": password}, "routes": routes}}
}

func (s *supervisor) bin(name string) string {
	return filepath.Join(s.opts.bundle, "bin/linux-amd64", name)
}
func (s *supervisor) start(label, name string, args ...string) (*child, error) {
	log, err := os.OpenFile(filepath.Join(s.opts.output, label+".log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(s.bin(name), args...)
	cmd.Dir = s.opts.output
	cmd.Env = append(os.Environ(), s.env...)
	cmd.Stdout = log
	cmd.Stderr = log
	if err = cmd.Start(); err != nil {
		log.Close()
		return nil, err
	}
	c := &child{cmd: cmd, done: make(chan struct{})}
	s.children = append(s.children, c)
	go func() { c.err = cmd.Wait(); log.Close(); close(c.done) }()
	return c, nil
}
func (s *supervisor) sync(ctx context.Context, label, name string, args ...string) error {
	c, err := s.start(label, name, args...)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Minute):
		return fmt.Errorf("%s timed out", label)
	case <-c.done:
		if c.err != nil {
			return fmt.Errorf("%s failed: %w (see private log)", label, c.err)
		}
		return nil
	}
}
func (s *supervisor) stop() {
	for i := len(s.children) - 1; i >= 0; i-- {
		c := s.children[i]
		select {
		case <-c.done:
			continue
		default:
		}
		_ = c.cmd.Process.Signal(os.Interrupt)
		select {
		case <-c.done:
		case <-time.After(2 * time.Second):
			_ = c.cmd.Process.Kill()
			<-c.done
		}
	}
}
func (s *supervisor) checkPermanent() error {
	for _, c := range s.permanent {
		select {
		case <-c.done:
			return fmt.Errorf("test service PID %d exited", c.cmd.Process.Pid)
		default:
		}
	}
	return nil
}
func (s *supervisor) get(url string, target any) error {
	r, err := s.client.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return fmt.Errorf("HTTP %d from loopback endpoint", r.StatusCode)
	}
	if target != nil {
		return json.NewDecoder(r.Body).Decode(target)
	}
	return nil
}
func (s *supervisor) nodesHealthy() error {
	for i := 0; i < 3; i++ {
		var v struct {
			ID    string `json:"server_id"`
			Start string `json:"start"`
		}
		base := fmt.Sprintf("http://127.0.0.1:%d", s.opts.port+6+i)
		if err := s.get(base+"/healthz?js-enabled-only=true", nil); err != nil {
			return err
		}
		if err := s.get(base+"/varz", &v); err != nil {
			return err
		}
		if v.ID == "" || v.Start == "" {
			return errors.New("node identity unavailable")
		}
		id := v.ID + "/" + v.Start
		if previous := s.ids[i]; previous != "" && previous != id {
			return errors.New("node identity changed")
		}
		s.ids[i] = id
	}
	return nil
}
func availablePorts(base int) error {
	var sockets []net.Listener
	defer func() {
		for _, s := range sockets {
			s.Close()
		}
	}()
	for i := 0; i < 10; i++ {
		l, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", base+i))
		if err != nil {
			return fmt.Errorf("test port unavailable; existing listener untouched: %w", err)
		}
		sockets = append(sockets, l)
	}
	return nil
}
func save(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(append(raw, '\n')); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
func checkReport(path string, duration time.Duration) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var r struct {
		Published, Consumed, Missing, Duplicates, Corrupt int64
		Requested                                         int64   `json:"requested_messages"`
		PublishRetries                                    int64   `json:"publish_retries"`
		ConsumeRetries                                    int64   `json:"consume_retries"`
		Publish                                           float64 `json:"publish_messages_per_second"`
		Consume                                           float64 `json:"consume_messages_per_second"`
		P99                                               float64 `json:"publish_latency_p99_millis"`
		Duration                                          float64 `json:"duration_seconds"`
		Replicas                                          int
	}
	if err = json.Unmarshal(raw, &r); err != nil {
		return err
	}
	if r.Requested < 1 || r.Published != r.Requested || r.Consumed != r.Requested || r.Missing != 0 || r.Corrupt != 0 || r.Duplicates != 0 || r.PublishRetries != 0 || r.ConsumeRetries != 0 || r.Replicas != 3 || r.Publish < 4900 || r.Consume < 4900 || r.P99 <= 0 || r.P99 > 10 || r.Duration < duration.Seconds() {
		return errors.New("integrity, duration or absolute 4900 msg/s / 10 ms gate failed")
	}
	return nil
}

func checkHost(path string) error {
	return checkHostWith(path, os.ReadFile, func(path string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return exec.CommandContext(ctx, "df", "-B1", "--output=avail", path).Output()
	})
}

func checkHostWith(path string, readFile func(string) ([]byte, error), diskFree func(string) ([]byte, error)) error {
	data, err := readFile("/proc/meminfo")
	if err != nil {
		return err
	}
	var available uint64
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) == 3 && f[0] == "MemAvailable:" {
			available, _ = strconv.ParseUint(f[1], 10, 64)
		}
	}
	if available < 16<<20 {
		return errors.New("host available memory below 16 GiB; stopping only qualification")
	}
	raw, err := diskFree(path)
	if err != nil {
		return err
	}
	return checkFreeDisk(raw)
}

func checkFreeDisk(raw []byte) error {
	f := strings.Fields(string(raw))
	if len(f) != 2 {
		return errors.New("cannot read host free storage")
	}
	free, err := strconv.ParseUint(f[1], 10, 64)
	if err != nil || free < 30<<30 {
		return errors.New("host free disk below 30 GiB; stopping only qualification")
	}
	return nil
}
