package main

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestVerifyChecksums(t *testing.T) {
	directory := t.TempDir()
	data := []byte("release artifact")
	if err := os.WriteFile(filepath.Join(directory, "artifact"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	manifest := []byte("manifest")
	manifestSum := sha256.Sum256(manifest)
	if err := os.WriteFile(filepath.Join(directory, "release-manifest.json"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	line := fmt.Sprintf("%x  artifact\n%x  release-manifest.json\n", sum, manifestSum)
	if err := os.WriteFile(filepath.Join(directory, "SHA256SUMS"), []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	count, err := verifyChecksums(directory)
	if err != nil || count != 2 {
		t.Fatalf("verifyChecksums() = %d, %v", count, err)
	}
	if err := os.WriteFile(filepath.Join(directory, "artifact"), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyChecksums(directory); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("tampered artifact was accepted: %v", err)
	}
}

func TestVerifyManifestArtifacts(t *testing.T) {
	directory := t.TempDir()
	manifest := releaseManifest{Version: "v0.1.0-rc.1"}
	for path := range expectedArtifactPaths(manifest.Version) {
		data := []byte("artifact: " + path)
		fullPath := filepath.Join(directory, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, data, 0o600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		manifest.Artifacts = append(manifest.Artifacts, releaseArtifact{Path: path, SHA256: fmt.Sprintf("%x", sum)})
	}
	if err := verifyManifestArtifacts(directory, manifest); err != nil {
		t.Fatal(err)
	}
	manifest.Artifacts[0].SHA256 = strings.Repeat("0", 64)
	if err := verifyManifestArtifacts(directory, manifest); err == nil || !strings.Contains(err.Error(), "manifest checksum mismatch") {
		t.Fatalf("bad manifest checksum was accepted: %v", err)
	}
}

func TestRC2RequiresLicenseRecords(t *testing.T) {
	old := expectedArtifactPaths("v0.1.0-rc.1")
	current := expectedArtifactPaths("v0.1.0-rc.2")
	for _, path := range []string{"licenses/NATS-LICENSE", "licenses/release-license-records.md", "licenses/release-license-records.zh-CN.md"} {
		if old[path] || !current[path] {
			t.Fatalf("incorrect license boundary for %s", path)
		}
	}
	manifest := releaseManifest{Version: "v0.1.0-rc.2"}
	for path := range current {
		if !strings.HasPrefix(path, "licenses/") {
			manifest.Artifacts = append(manifest.Artifacts, releaseArtifact{Path: path})
		}
	}
	if err := verifyManifestArtifacts(t.TempDir(), manifest); err == nil {
		t.Fatal("rc.2 accepted missing license records")
	}
}

func TestVerifyChecksumsRejectsTraversal(t *testing.T) {
	directory := t.TempDir()
	line := strings.Repeat("0", 64) + "  ../outside\n"
	if err := os.WriteFile(filepath.Join(directory, "SHA256SUMS"), []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyChecksums(directory); err == nil || !strings.Contains(err.Error(), "invalid checksum entry") {
		t.Fatalf("traversal entry was accepted: %v", err)
	}
}

func TestReadOCIPlatforms(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.oci.tar")
	writeTestOCI(t, path)
	platforms, err := readOCIPlatforms(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(platforms, ",") != "linux/amd64,linux/arm64" {
		t.Fatalf("platforms = %v", platforms)
	}
}

func writeTestOCI(t *testing.T, path string) {
	t.Helper()
	inner := []byte(`{"schemaVersion":2,"manifests":[{"platform":{"os":"linux","architecture":"arm64"}},{"platform":{"os":"unknown","architecture":"unknown"}},{"platform":{"os":"linux","architecture":"amd64"}}]}`)
	digest := sha256.Sum256(inner)
	root := []byte(fmt.Sprintf(`{"schemaVersion":2,"manifests":[{"digest":"sha256:%x"}]}`, digest))
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := tar.NewWriter(file)
	for name, data := range map[string][]byte{"index.json": root, fmt.Sprintf("blobs/sha256/%x", digest): inner} {
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestWriteExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evidence.json")
	if err := writeExclusive(path, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := writeExclusive(path, []byte("second")); err == nil {
		t.Fatal("existing evidence was overwritten")
	}
}

func TestRunRequiresPaths(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr); code != 2 {
		t.Fatalf("run() = %d, stderr=%s", code, stderr.String())
	}
}

func TestRunWritesEvidenceExclusively(t *testing.T) {
	directory := t.TempDir()
	output := filepath.Join(directory, "evidence.json")
	qualified := evidence{Schema: evidenceSchema, SourceRevision: strings.Repeat("a", 40)}
	qualifier := func(bundle, revision string) (evidence, error) {
		if bundle != "bundle" || revision != qualified.SourceRevision {
			t.Fatalf("qualifier arguments = %q, %q", bundle, revision)
		}
		return qualified, nil
	}
	args := []string{"-bundle", "bundle", "-output", output, "-source-revision", qualified.SourceRevision}
	var stdout, stderr bytes.Buffer
	if code := runWithQualifier(args, &stdout, &stderr, qualifier); code != 0 {
		t.Fatalf("runWithQualifier() = %d, stderr=%s", code, stderr.String())
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var written evidence
	if err := json.Unmarshal(data, &written); err != nil || written.Schema != evidenceSchema {
		t.Fatalf("written evidence = %+v, %v", written, err)
	}
	if code := runWithQualifier(args, &stdout, &stderr, qualifier); code != 1 || !strings.Contains(stderr.String(), "create evidence") {
		t.Fatalf("existing evidence was overwritten: code=%d stderr=%s", code, stderr.String())
	}
	failure := func(string, string) (evidence, error) { return evidence{}, errors.New("qualification failed") }
	if code := runWithQualifier([]string{"-bundle", "bundle", "-output", filepath.Join(directory, "failed.json"), "-source-revision", qualified.SourceRevision}, &stdout, &stderr, failure); code != 1 || !strings.Contains(stderr.String(), "qualification failed") {
		t.Fatalf("qualification error was lost: code=%d stderr=%s", code, stderr.String())
	}
}

func TestQualifyWithEnvironment(t *testing.T) {
	bundle, revision := createTestBundle(t)
	fixedTime := time.Date(2026, 8, 2, 1, 2, 3, 0, time.UTC)
	environment := qualificationEnvironment{
		goos: "linux", goarch: "amd64", now: func() time.Time { return fixedTime },
		readFile: func(path string) ([]byte, error) {
			switch path {
			case "/proc/sys/kernel/osrelease":
				return []byte("6.12.0-production\n"), nil
			case "/proc/version":
				return []byte("Linux production"), nil
			default:
				return nil, os.ErrNotExist
			}
		},
		command: func(name string, args ...string) (string, error) {
			switch {
			case name == "docker":
				return `{"Name":"native-host","OperatingSystem":"Ubuntu 24.04","OSType":"linux","Architecture":"x86_64","KernelVersion":"6.12","Driver":"overlay2","DockerRootDir":"/var/lib/docker","NCPU":16,"MemTotal":34359738368}`, nil
			case filepath.Base(name) == "rjsctl" || filepath.Base(name) == "rjs-management":
				return "v0.1.0-rc.1", nil
			default:
				return "", fmt.Errorf("unexpected command: %s %v", name, args)
			}
		},
	}
	result, err := qualifyWithEnvironment(bundle, revision, environment)
	if err != nil {
		t.Fatal(err)
	}
	if result.Schema != evidenceSchema || result.Runtime != "linux/amd64" || result.SourceRevision != revision || result.ChecksumFiles != 13 || len(result.Images) != 3 || result.GeneratedAt != fixedTime.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected evidence: %+v", result)
	}

	environment.goos = "windows"
	if _, err := qualifyWithEnvironment(bundle, revision, environment); err == nil || !strings.Contains(err.Error(), "native Linux is required") {
		t.Fatalf("non-Linux environment was accepted: %v", err)
	}
	environment.goos, environment.goarch = "linux", "386"
	if _, err := qualifyWithEnvironment(bundle, revision, environment); err == nil || !strings.Contains(err.Error(), "requires amd64") {
		t.Fatalf("unsupported architecture was accepted: %v", err)
	}
	environment.goarch = "amd64"
	if _, err := qualifyWithEnvironment(bundle, "bad", environment); err == nil || !strings.Contains(err.Error(), "invalid expected source revision") {
		t.Fatalf("invalid expected revision was accepted: %v", err)
	}
}

func createTestBundle(t *testing.T) (string, string) {
	t.Helper()
	bundle := t.TempDir()
	revision := strings.Repeat("a", 40)
	manifest := releaseManifest{Version: "v0.1.0-rc.1", ServerRevision: revision, Qualification: "local-release-gates-passed; native-linux-soak-and-canary-required", ContractVersion: "v1alpha1", NATSVersion: "v2.14.1"}
	manifest.SDK.Version = "0.1.0-rc.1"
	manifest.SDK.Revision = strings.Repeat("b", 40)
	paths := make([]string, 0, len(expectedArtifactPaths(manifest.Version)))
	for path := range expectedArtifactPaths(manifest.Version) {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		fullPath := filepath.Join(bundle, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(path, ".oci.tar") {
			writeTestOCI(t, fullPath)
		} else if err := os.WriteFile(fullPath, []byte("artifact: "+path), 0o700); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		manifest.Artifacts = append(manifest.Artifacts, releaseArtifact{Path: path, SHA256: fmt.Sprintf("%x", sum)})
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(bundle, "release-manifest.json")
	if err := os.WriteFile(manifestPath, manifestData, 0o600); err != nil {
		t.Fatal(err)
	}
	checksumPaths := append(append([]string{}, paths...), "release-manifest.json")
	sort.Strings(checksumPaths)
	var checksumFile strings.Builder
	for _, path := range checksumPaths {
		data, err := os.ReadFile(filepath.Join(bundle, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		fmt.Fprintf(&checksumFile, "%x  %s\n", sum, path)
	}
	if err := os.WriteFile(filepath.Join(bundle, "SHA256SUMS"), []byte(checksumFile.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return bundle, revision
}

func TestReadManifestAndCommandOutput(t *testing.T) {
	bundle, _ := createTestBundle(t)
	manifest, err := readManifest(filepath.Join(bundle, "release-manifest.json"))
	if err != nil || manifest.Version != "v0.1.0-rc.1" {
		t.Fatalf("readManifest() = %+v, %v", manifest, err)
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readManifest(bad); err == nil {
		t.Fatal("incomplete manifest was accepted")
	}
	if output, err := commandOutput("go", "version"); err != nil || !strings.HasPrefix(output, "go version") {
		t.Fatalf("commandOutput(go version) = %q, %v", output, err)
	}
	if _, err := commandOutput(filepath.Join(t.TempDir(), "missing-command")); err == nil {
		t.Fatal("missing command succeeded")
	}
}

func TestReadDockerInfoRejectsInvalidJSON(t *testing.T) {
	_, err := readDockerInfo(func(string, ...string) (string, error) { return "not-json", nil })
	if err == nil || !strings.Contains(err.Error(), "decode Docker server info") {
		t.Fatalf("invalid Docker info was accepted: %v", err)
	}
	_, err = readDockerInfo(func(string, ...string) (string, error) { return "", errors.New("unavailable") })
	if err == nil || !strings.Contains(err.Error(), "inspect Docker server") {
		t.Fatalf("Docker command error was lost: %v", err)
	}
}

func TestReadContainerInfoFallsBackToPodman(t *testing.T) {
	result, err := readContainerInfo(func(name string, _ ...string) (string, error) {
		if name == "docker" {
			return "", errors.New("not installed")
		}
		return `{"host":{"arch":"amd64","cpus":2,"hostname":"native-host","kernel":"6.12","memTotal":4096,"os":"linux","distribution":{"distribution":"rocky","version":"9.8"}},"store":{"graphDriverName":"overlay","graphRoot":"/home/sandbox/.local/share/containers/storage"}}`, nil
	})
	if err != nil || result.Engine != "podman" || result.OSType != "linux" || result.Architecture != "amd64" || result.Driver != "overlay" {
		t.Fatalf("Podman info = %+v, %v", result, err)
	}
}

func TestValidRevision(t *testing.T) {
	if !validRevision(strings.Repeat("a", 40)) || validRevision("bad") {
		t.Fatal("revision validation failed")
	}
}
