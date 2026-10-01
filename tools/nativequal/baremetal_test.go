package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func bareFixture(t *testing.T) (string, releaseManifest, qualificationEnvironment) {
	t.Helper()
	dir := t.TempDir()
	m := releaseManifest{DeploymentMode: "bare-metal", ServerRevision: strings.Repeat("a", 40), VerificationRevision: strings.Repeat("b", 40), Version: "v0.1.0-rc.2", NATSVersion: "v2.14.1", Qualification: "native-linux-soak-and-canary-required", ContractVersion: "v1alpha1"}
	m.SDK.Version = "0.1.0-rc.2"
	m.SDK.Revision = strings.Repeat("c", 40)
	for _, name := range []string{"nats-server", "rjs-management", "rjsctl", "nativequal", "jetstream-bench", "resource-sampler", "perfevidence", "resourceaudit", "baremetal-run"} {
		path := "bin/linux-amd64/" + name
		full := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		data := []byte(name)
		if err := os.WriteFile(full, data, 0600); err != nil {
			t.Fatal(err)
		}
		m.Artifacts = append(m.Artifacts, releaseArtifact{Path: path, SHA256: fmt.Sprintf("%x", sha256.Sum256(data))})
	}
	for _, name := range []string{"NATS-LICENSE", "release-license-records.md", "release-license-records.zh-CN.md"} {
		path := "licenses/" + name
		if err := os.MkdirAll(filepath.Join(dir, "licenses"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, path), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
		m.Artifacts = append(m.Artifacts, releaseArtifact{Path: path, SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(name)))})
	}
	writeBareManifest(t, dir, m)
	env := qualificationEnvironment{goos: "linux", goarch: "amd64", now: time.Now, readFile: func(path string) ([]byte, error) {
		v, ok := map[string]string{"/proc/sys/kernel/osrelease": "6.12.0", "/proc/version": "Linux native", "/proc/meminfo": "MemTotal: 64000000 kB\n", "/proc/stat": "cpu 1 2\ncpu0 1 2\ncpu1 1 2\n", "/etc/os-release": "PRETTY_NAME=\"Oracle Linux\"\n"}[path]
		if !ok {
			return nil, os.ErrNotExist
		}
		return []byte(v), nil
	}, command: func(name string, args ...string) (string, error) {
		switch filepath.Base(name) {
		case "id":
			return "1000", nil
		case "findmnt":
			return "xfs /dev/test", nil
		case "nats-server":
			return "nats-server: v2.14.1", nil
		case "rjsctl", "rjs-management":
			return "v0.1.0-rc.2", nil
		default:
			t.Fatalf("unexpected command: %s", name)
			return "", nil
		}
	}}
	return dir, m, env
}
func writeBareManifest(t *testing.T, dir string, m releaseManifest) {
	t.Helper()
	raw, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(dir, "release-manifest.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	sums := fmt.Sprintf("%x  release-manifest.json\n", sha256.Sum256(raw))
	for _, a := range m.Artifacts {
		sums += a.SHA256 + "  " + a.Path + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(sums), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestBareMetalQualification(t *testing.T) {
	dir, m, env := bareFixture(t)
	e, err := qualifyBareMetal(dir, m.ServerRevision, t.TempDir(), env)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(e)
	if strings.Contains(string(raw), `"docker"`) || e.DeploymentMode != "bare-metal" || len(e.NATSBinarySHA256) != 64 || e.Host["uid"] != 1000 || e.VerificationRevision != m.VerificationRevision {
		t.Fatalf("bad evidence: %s", raw)
	}
}
func TestBareMetalRejectsUnsafeInputs(t *testing.T) {
	for _, kind := range []string{"root", "container", "wsl", "filesystem", "cpu", "memory", "revision", "tamper", "duplicate", "missing", "traversal", "version", "platform"} {
		t.Run(kind, func(t *testing.T) {
			dir, m, env := bareFixture(t)
			read, command := env.readFile, env.command
			env.readFile = func(path string) ([]byte, error) {
				if kind == "container" && path == "/.dockerenv" {
					return []byte{}, nil
				}
				if kind == "wsl" && path == "/proc/version" {
					return []byte("microsoft WSL"), nil
				}
				if kind == "cpu" && path == "/proc/stat" {
					return []byte{}, nil
				}
				if kind == "memory" && path == "/proc/meminfo" {
					return []byte("MemTotal: -1 kB"), nil
				}
				return read(path)
			}
			env.command = func(name string, args ...string) (string, error) {
				if kind == "root" && name == "id" {
					return "0", nil
				}
				if kind == "filesystem" && name == "findmnt" {
					return "tmpfs tmpfs", nil
				}
				if kind == "version" && filepath.Base(name) == "nats-server" {
					return "other", nil
				}
				return command(name, args...)
			}
			switch kind {
			case "revision":
				m.ServerRevision = strings.Repeat("d", 40)
			case "tamper":
				m.Artifacts[0].SHA256 = strings.Repeat("0", 64)
			case "duplicate":
				m.Artifacts = append(m.Artifacts, m.Artifacts[0])
			case "missing":
				m.Artifacts = m.Artifacts[1:]
			case "traversal":
				m.Artifacts[0].Path = "../outside"
			case "platform":
				env.goos = "windows"
			}
			writeBareManifest(t, dir, m)
			if _, err := qualifyBareMetal(dir, strings.Repeat("a", 40), t.TempDir(), env); err == nil {
				t.Fatal("unsafe qualification accepted")
			}
		})
	}
}
