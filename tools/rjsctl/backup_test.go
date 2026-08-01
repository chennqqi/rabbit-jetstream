package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

type fakeCommandRunner struct {
	streams  []string
	existing []string
	calls    [][]string
	failAt   int
}

func (f *fakeCommandRunner) Run(_ string, args []string, stdout, _ io.Writer) error {
	f.calls = append(f.calls, slices.Clone(args))
	if f.failAt > 0 && len(f.calls) == f.failAt {
		return errors.New("command failed")
	}
	if slices.Contains(args, "--version") {
		_, err := io.WriteString(stdout, "0.4.0\n")
		return err
	}
	if slices.Contains(args, "ls") {
		value := f.streams
		if slices.Contains(args, "destination.invalid") {
			value = f.existing
		}
		return json.NewEncoder(stdout).Encode(value)
	}
	if index := slices.Index(args, "backup"); index >= 0 {
		target := args[len(args)-1]
		if err := os.WriteFile(filepath.Join(target, "backup.json"), []byte(`{"stream":"`+args[len(args)-2]+`"}`), 0o600); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(target, "stream.tar.s2"), []byte("snapshot"), 0o600)
	}
	return nil
}

func TestCreateVerifyAndRestoreBackup(t *testing.T) {
	runner := &fakeCommandRunner{streams: []string{"RJSQ_orders", "KV_RJS_META"}}
	output := filepath.Join(t.TempDir(), "backup")
	now := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	if err := createBackup(runner, "nats", "nats://nats:4222", output, now, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	manifest, err := verifyBackup(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Streams) != 2 || manifest.Streams[0].Name != "KV_RJS_META" || manifest.Server != "nats://nats:4222" || manifest.NATSCLI != "0.4.0" || manifest.FileCount != 4 || manifest.TotalBytes == 0 {
		t.Fatalf("manifest=%#v", manifest)
	}
	restore := &fakeCommandRunner{streams: []string{}}
	if err := restoreBackup(restore, "nats", "nats://destination.invalid:4222", output, "RESTORE", 1, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if len(restore.calls) != 3 {
		t.Fatalf("restore calls=%v", restore.calls)
	}
	for _, call := range restore.calls[1:] {
		if !slices.Contains(call, "--replicas") || !slices.Contains(call, "1") || !slices.Contains(call, "--no-progress") {
			t.Fatalf("restore call=%v", call)
		}
	}
}

func TestCreateBackupIsTransactional(t *testing.T) {
	parent := t.TempDir()
	output := filepath.Join(parent, "backup")
	runner := &fakeCommandRunner{streams: []string{"RJSQ_a", "RJSQ_b"}, failAt: 3}
	if err := createBackup(runner, "nats", "", output, time.Now(), io.Discard, io.Discard); err == nil {
		t.Fatal("failed backup succeeded")
	}
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output exists: %v", err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("staging entries=%v err=%v", entries, err)
	}
	if err := os.Mkdir(output, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := createBackup(runner, "nats", "", output, time.Now(), io.Discard, io.Discard); err == nil {
		t.Fatal("overwrite succeeded")
	}
}

func TestVerifyBackupDetectsTampering(t *testing.T) {
	output := filepath.Join(t.TempDir(), "backup")
	if err := createBackup(&fakeCommandRunner{streams: []string{"RJSQ_orders"}}, "nats", "", output, time.Now(), io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(output, "streams", "RJSQ_orders", "stream.tar.s2")
	if err := os.WriteFile(path, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyBackup(output); err == nil || !strings.Contains(err.Error(), "integrity") {
		t.Fatalf("err=%v", err)
	}
}

func TestVerifyBackupRejectsUnlistedFiles(t *testing.T) {
	output := filepath.Join(t.TempDir(), "backup")
	if err := createBackup(&fakeCommandRunner{streams: []string{"RJSQ_orders"}}, "nats", "", output, time.Now(), io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "streams", "RJSQ_orders", "unexpected"), []byte("not listed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyBackup(output); err == nil || !strings.Contains(err.Error(), "not declared") {
		t.Fatalf("err=%v", err)
	}
}

func TestRestoreSafetyChecks(t *testing.T) {
	output := filepath.Join(t.TempDir(), "backup")
	if err := createBackup(&fakeCommandRunner{streams: []string{"RJSQ_orders"}}, "nats", "", output, time.Now(), io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		confirm  string
		replicas int
	}{{"", 1}, {"RESTORE", 2}} {
		if err := restoreBackup(&fakeCommandRunner{}, "nats", "", output, test.confirm, test.replicas, io.Discard, io.Discard); err == nil {
			t.Fatalf("confirm=%q replicas=%d succeeded", test.confirm, test.replicas)
		}
	}
	runner := &fakeCommandRunner{streams: []string{"RJSQ_orders"}}
	if err := restoreBackup(runner, "nats", "", output, "RESTORE", 0, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err=%v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("restore started before collision check: %v", runner.calls)
	}
}

func TestBackupCommands(t *testing.T) {
	for _, args := range [][]string{{"backup"}, {"backup", "unknown"}, {"backup", "create"}, {"backup", "verify"}, {"backup", "restore"}} {
		if err := run(args, io.Discard, io.Discard); err == nil {
			t.Fatalf("args=%v succeeded", args)
		}
	}
	output := filepath.Join(t.TempDir(), "backup")
	if err := createBackup(&fakeCommandRunner{}, "nats", "", output, time.Now(), io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := run([]string{"backup", "verify", "--input", output}, &stdout, io.Discard); err != nil || !strings.Contains(stdout.String(), "verified 0 Streams") {
		t.Fatalf("stdout=%q err=%v", stdout.String(), err)
	}
}

func TestBackupHelpers(t *testing.T) {
	for _, name := range []string{"", ".", "..", "bad.name", "bad/name", "bad name"} {
		if safeStreamName(name) {
			t.Fatalf("unsafe name accepted: %q", name)
		}
	}
	if !safeStreamName("KV_RJS_META") {
		t.Fatal("safe name rejected")
	}
	if got := natsArgs("", "stream", "ls"); !slices.Equal(got, []string{"stream", "ls"}) {
		t.Fatalf("args=%v", got)
	}
	for _, server := range []string{"relative", "nats://user:secret@nats:4222"} {
		if err := validateBackupServer(server); err == nil {
			t.Fatalf("server %q accepted", server)
		}
	}
	if err := validateBackupServer("nats://nats:4222"); err != nil {
		t.Fatal(err)
	}
}

func TestNATSCLIEnvironmentBridgesRJSConnectionSettingsWithoutOverridingStandardValues(t *testing.T) {
	values := map[string]string{
		"RJS_NATS_URL": "tls://nats:4222", "RJS_NATS_USER": "rjs", "RJS_NATS_PASSWORD": "secret",
		"RJS_NATS_CREDS": "/tls/user.creds", "RJS_NATS_TLS_CA": "/tls/ca.crt", "RJS_NATS_TLS_CERT": "/tls/client.crt", "RJS_NATS_TLS_KEY": "/tls/client.key",
	}
	got := natsCLIEnvironment([]string{"PATH=/bin", "NATS_USER=standard"}, func(key string) string { return values[key] })
	joined := strings.Join(got, "\n")
	for _, expected := range []string{"NATS_URL=tls://nats:4222", "NATS_USER=standard", "NATS_CA=/tls/ca.crt", "NATS_CERT=/tls/client.crt", "NATS_KEY=/tls/client.key"} {
		if strings.Count(joined, expected) != 1 {
			t.Fatalf("environment missing or duplicates %q: %v", expected, got)
		}
	}
	if strings.Contains(joined, "NATS_USER=rjs") {
		t.Fatalf("standard NATS_USER was overridden: %v", got)
	}
	if strings.Contains(joined, "NATS_PASSWORD=secret") || strings.Contains(joined, "NATS_CREDS=/tls/user.creds") {
		t.Fatalf("RJS authentication was mixed with standard authentication: %v", got)
	}
	credentialsOnly := natsCLIEnvironment(nil, func(key string) string { return values[key] })
	credentialsJoined := strings.Join(credentialsOnly, "\n")
	if !strings.Contains(credentialsJoined, "NATS_CREDS=/tls/user.creds") || strings.Contains(credentialsJoined, "NATS_USER=") || strings.Contains(credentialsJoined, "NATS_PASSWORD=") {
		t.Fatalf("credentials did not take precedence: %v", credentialsOnly)
	}
}
