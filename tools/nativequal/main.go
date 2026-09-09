package main

import (
	"archive/tar"
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const evidenceSchema = "rabbit-jetstream.io/native-linux-preflight/v1alpha1"

type releaseManifest struct {
	DeploymentMode       string `json:"deployment_mode,omitempty"`
	VerificationRevision string `json:"verification_revision,omitempty"`
	Version              string `json:"version"`
	ServerRevision       string `json:"server_revision"`
	Qualification        string `json:"qualification"`
	ContractVersion      string `json:"contract_version"`
	NATSVersion          string `json:"nats_version"`
	SDK                  struct {
		Version  string `json:"version"`
		Revision string `json:"revision"`
	} `json:"sdk"`
	Artifacts []releaseArtifact `json:"artifacts"`
}

type releaseArtifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type dockerInfo struct {
	Engine          string `json:"Engine"`
	Name            string `json:"Name"`
	OperatingSystem string `json:"OperatingSystem"`
	OSType          string `json:"OSType"`
	Architecture    string `json:"Architecture"`
	KernelVersion   string `json:"KernelVersion"`
	Driver          string `json:"Driver"`
	DockerRootDir   string `json:"DockerRootDir"`
	NCPU            int    `json:"NCPU"`
	MemTotal        int64  `json:"MemTotal"`
}

type platformResult struct {
	Archive   string   `json:"archive"`
	Platforms []string `json:"platforms"`
}

type evidence struct {
	Schema               string            `json:"schema"`
	GeneratedAt          string            `json:"generated_at"`
	SourceRevision       string            `json:"source_revision"`
	ReleaseVersion       string            `json:"release_version"`
	SDKVersion           string            `json:"sdk_version"`
	SDKRevision          string            `json:"sdk_revision"`
	ContractVersion      string            `json:"contract_version"`
	NATSVersion          string            `json:"nats_version"`
	Runtime              string            `json:"runtime"`
	KernelRelease        string            `json:"kernel_release"`
	Docker               dockerInfo        `json:"docker,omitzero"`
	DeploymentMode       string            `json:"deployment_mode,omitempty"`
	Host                 map[string]any    `json:"host,omitempty"`
	NATSBinarySHA256     string            `json:"nats_binary_sha256,omitempty"`
	VerificationRevision string            `json:"verification_revision,omitempty"`
	BundleManifestSHA256 string            `json:"bundle_manifest_sha256,omitempty"`
	ChecksumFiles        int               `json:"checksum_files"`
	Images               []platformResult  `json:"images"`
	Binaries             map[string]string `json:"binaries"`
}

type qualificationEnvironment struct {
	goos     string
	goarch   string
	readFile func(string) ([]byte, error)
	command  func(string, ...string) (string, error)
	now      func() time.Time
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	return runWithQualifier(args, stdout, stderr, qualify)
}

func runWithQualifier(args []string, stdout, stderr io.Writer, qualifier func(string, string) (evidence, error)) int {
	flags := flag.NewFlagSet("nativequal", flag.ContinueOnError)
	flags.SetOutput(stderr)
	bundle := flags.String("bundle", "", "local release bundle directory")
	output := flags.String("output", "", "preflight evidence JSON output")
	expectedRevision := flags.String("source-revision", "", "expected 40-character server revision")
	mode := flags.String("mode", "container", "container or bare-metal deployment")
	dataPath := flags.String("data-path", "", "existing bare-metal data filesystem path")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *bundle == "" || *output == "" || !validRevision(*expectedRevision) {
		fmt.Fprintln(stderr, "-bundle, -output and a 40-character -source-revision are required")
		return 2
	}
	if *mode != "container" && *mode != "bare-metal" || *mode == "bare-metal" && *dataPath == "" {
		fmt.Fprintln(stderr, "valid -mode and bare-metal -data-path are required")
		return 2
	}
	var result evidence
	var err error
	if *mode == "bare-metal" {
		result, err = qualifyBareMetal(*bundle, *expectedRevision, *dataPath, qualificationEnvironment{goos: runtime.GOOS, goarch: runtime.GOARCH, readFile: os.ReadFile, command: commandOutput, now: time.Now})
	} else {
		result, err = qualifier(*bundle, *expectedRevision)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	data = append(data, '\n')
	if err := writeExclusive(*output, data); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "native Linux preflight verified: %s\n", *output)
	return 0
}

func qualify(bundle, expectedRevision string) (evidence, error) {
	return qualifyWithEnvironment(bundle, expectedRevision, qualificationEnvironment{
		goos: runtime.GOOS, goarch: runtime.GOARCH, readFile: os.ReadFile,
		command: commandOutput, now: time.Now,
	})
}

func qualifyWithEnvironment(bundle, expectedRevision string, environment qualificationEnvironment) (evidence, error) {
	var result evidence
	if environment.goos != "linux" {
		return result, fmt.Errorf("native Linux is required; current runtime is %s/%s", environment.goos, environment.goarch)
	}
	if environment.goarch != "amd64" {
		return result, fmt.Errorf("first-release qualification requires amd64, got %s", environment.goarch)
	}
	kernel, err := environment.readFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return result, fmt.Errorf("read kernel release: %w", err)
	}
	version, err := environment.readFile("/proc/version")
	if err != nil {
		return result, fmt.Errorf("read kernel version: %w", err)
	}
	if strings.Contains(strings.ToLower(string(kernel)+string(version)), "microsoft") {
		return result, errors.New("WSL is not accepted for release qualification")
	}
	docker, err := readContainerInfo(environment.command)
	if err != nil {
		return result, err
	}
	if docker.OSType != "linux" {
		return result, fmt.Errorf("container engine host must be Linux, got %q", docker.OSType)
	}
	identity := strings.ToLower(docker.Name + " " + docker.OperatingSystem)
	if strings.Contains(identity, "docker desktop") {
		return result, errors.New("Docker Desktop is not accepted for release qualification")
	}
	if !validRevision(expectedRevision) {
		return result, fmt.Errorf("invalid expected source revision %q", expectedRevision)
	}
	bundle, err = filepath.Abs(bundle)
	if err != nil {
		return result, err
	}
	manifest, err := readManifest(filepath.Join(bundle, "release-manifest.json"))
	if err != nil {
		return result, err
	}
	if manifest.ServerRevision != expectedRevision {
		return result, fmt.Errorf("bundle revision %s does not match expected revision %s", manifest.ServerRevision, expectedRevision)
	}
	if !strings.Contains(manifest.Qualification, "native-linux-soak-and-canary-required") {
		return result, errors.New("bundle qualification boundary is missing")
	}
	if err := verifyManifestArtifacts(bundle, manifest); err != nil {
		return result, err
	}
	checksums, err := verifyChecksums(bundle)
	if err != nil {
		return result, err
	}
	if checksums != len(manifest.Artifacts)+1 {
		return result, fmt.Errorf("SHA256SUMS contains %d entries, expected %d", checksums, len(manifest.Artifacts)+1)
	}
	images := make([]platformResult, 0, 3)
	for _, name := range []string{"management.oci.tar", "nats.oci.tar", "operator.oci.tar"} {
		platforms, err := readOCIPlatforms(filepath.Join(bundle, "images", name))
		if err != nil {
			return result, fmt.Errorf("verify %s: %w", name, err)
		}
		if strings.Join(platforms, ",") != "linux/amd64,linux/arm64" {
			return result, fmt.Errorf("%s platforms are %v", name, platforms)
		}
		images = append(images, platformResult{Archive: name, Platforms: platforms})
	}
	binaries := make(map[string]string, 2)
	binDir := filepath.Join(bundle, "bin", "linux-"+environment.goarch)
	for name, args := range map[string][]string{"rjsctl": {"version"}, "rjs-management": {"--version"}} {
		value, err := environment.command(filepath.Join(binDir, name), args...)
		if err != nil {
			return result, fmt.Errorf("execute %s: %w", name, err)
		}
		if value != manifest.Version {
			return result, fmt.Errorf("%s reports %q, expected %q", name, value, manifest.Version)
		}
		binaries[name] = value
	}
	result = evidence{
		Schema: evidenceSchema, GeneratedAt: environment.now().UTC().Format(time.RFC3339Nano),
		SourceRevision: expectedRevision, ReleaseVersion: manifest.Version, SDKVersion: manifest.SDK.Version,
		SDKRevision: manifest.SDK.Revision, ContractVersion: manifest.ContractVersion,
		NATSVersion: manifest.NATSVersion, Runtime: environment.goos + "/" + environment.goarch,
		KernelRelease: strings.TrimSpace(string(kernel)), Docker: docker,
		ChecksumFiles: checksums, Images: images, Binaries: binaries,
	}
	return result, nil
}

func readDockerInfo(command func(string, ...string) (string, error)) (dockerInfo, error) {
	var result dockerInfo
	raw, err := command("docker", "info", "--format", "{{json .}}")
	if err != nil {
		return result, fmt.Errorf("inspect Docker server: %w", err)
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return result, fmt.Errorf("decode Docker server info: %w", err)
	}
	result.Engine = "docker"
	return result, nil
}

func readContainerInfo(command func(string, ...string) (string, error)) (dockerInfo, error) {
	result, dockerErr := readDockerInfo(command)
	if dockerErr == nil {
		return result, nil
	}
	raw, podmanErr := command("podman", "info", "--format", "json")
	if podmanErr != nil {
		return result, fmt.Errorf("inspect container engine (Docker: %v; Podman: %w)", dockerErr, podmanErr)
	}
	var info struct {
		Host struct {
			Arch         string `json:"arch"`
			CPUs         int    `json:"cpus"`
			Hostname     string `json:"hostname"`
			Kernel       string `json:"kernel"`
			MemTotal     int64  `json:"memTotal"`
			OS           string `json:"os"`
			Distribution struct {
				Name    string `json:"distribution"`
				Version string `json:"version"`
			} `json:"distribution"`
		} `json:"host"`
		Store struct {
			Driver string `json:"graphDriverName"`
			Root   string `json:"graphRoot"`
		} `json:"store"`
	}
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return result, fmt.Errorf("decode Podman host info: %w", err)
	}
	if info.Host.OS == "" || info.Host.Arch == "" {
		return result, errors.New("Podman host info is incomplete")
	}
	return dockerInfo{
		Engine: "podman", Name: info.Host.Hostname,
		OperatingSystem: strings.TrimSpace(info.Host.Distribution.Name + " " + info.Host.Distribution.Version),
		OSType:          info.Host.OS, Architecture: info.Host.Arch, KernelVersion: info.Host.Kernel,
		Driver: info.Store.Driver, DockerRootDir: info.Store.Root, NCPU: info.Host.CPUs, MemTotal: info.Host.MemTotal,
	}, nil
}

func readManifest(path string) (releaseManifest, error) {
	var result releaseManifest
	data, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, err
	}
	if result.Version == "" || result.ServerRevision == "" || result.SDK.Version == "" || result.SDK.Revision == "" {
		return result, errors.New("release manifest is incomplete")
	}
	return result, nil
}

func verifyChecksums(bundle string) (int, error) {
	file, err := os.Open(filepath.Join(bundle, "SHA256SUMS"))
	if err != nil {
		return 0, err
	}
	defer file.Close()
	count := 0
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), "  ", 2)
		if len(parts) != 2 || len(parts[0]) != 64 || !safeRelativePath(parts[1]) || seen[parts[1]] {
			return 0, fmt.Errorf("invalid checksum entry %q", scanner.Text())
		}
		seen[parts[1]] = true
		expected, err := hex.DecodeString(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid checksum for %s", parts[1])
		}
		path := filepath.Join(bundle, filepath.FromSlash(parts[1]))
		data, err := os.Open(path)
		if err != nil {
			return 0, err
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, data)
		closeErr := data.Close()
		if copyErr != nil {
			return 0, copyErr
		}
		if closeErr != nil {
			return 0, closeErr
		}
		if !equalBytes(expected, hash.Sum(nil)) {
			return 0, fmt.Errorf("checksum mismatch: %s", parts[1])
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, errors.New("SHA256SUMS is empty")
	}
	if !seen["release-manifest.json"] {
		return 0, errors.New("SHA256SUMS does not cover release-manifest.json")
	}
	return count, nil
}

func verifyManifestArtifacts(bundle string, manifest releaseManifest) error {
	expectedPaths := expectedArtifactPaths(manifest.Version)
	if len(manifest.Artifacts) != len(expectedPaths) {
		return fmt.Errorf("release manifest contains %d artifacts, expected %d", len(manifest.Artifacts), len(expectedPaths))
	}
	seen := make(map[string]bool, len(manifest.Artifacts))
	for _, artifact := range manifest.Artifacts {
		if !safeRelativePath(artifact.Path) || seen[artifact.Path] || len(artifact.SHA256) != 64 {
			return fmt.Errorf("invalid manifest artifact %q", artifact.Path)
		}
		seen[artifact.Path] = true
		if !expectedPaths[artifact.Path] {
			return fmt.Errorf("unexpected manifest artifact %q", artifact.Path)
		}
		expected, err := hex.DecodeString(artifact.SHA256)
		if err != nil {
			return fmt.Errorf("invalid manifest checksum for %s", artifact.Path)
		}
		file, err := os.Open(filepath.Join(bundle, filepath.FromSlash(artifact.Path)))
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if !equalBytes(expected, hash.Sum(nil)) {
			return fmt.Errorf("manifest checksum mismatch: %s", artifact.Path)
		}
	}
	return nil
}

func expectedArtifactPaths(version string) map[string]bool {
	paths := map[string]bool{
		"bin/linux-amd64/rjs-management":                                true,
		"bin/linux-amd64/rjsctl":                                        true,
		"bin/linux-amd64/nativequal":                                    true,
		"bin/linux-arm64/rjs-management":                                true,
		"bin/linux-arm64/rjsctl":                                        true,
		"bin/linux-arm64/nativequal":                                    true,
		"images/management.oci.tar":                                     true,
		"images/nats.oci.tar":                                           true,
		"images/operator.oci.tar":                                       true,
		"evidence/local-rc.json":                                        true,
		"evidence/performance-ci.json":                                  true,
		"rabbit-jetstream-" + strings.TrimPrefix(version, "v") + ".tgz": true,
	}
	if version != "v0.1.0-rc.1" {
		paths["licenses/NATS-LICENSE"] = true
		paths["licenses/release-license-records.md"] = true
		paths["licenses/release-license-records.zh-CN.md"] = true
	}
	return paths
}

func safeRelativePath(path string) bool {
	if path == "" || filepath.IsAbs(path) || filepath.VolumeName(path) != "" || strings.Contains(path, "\\") {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	return clean == path && clean != "." && !strings.HasPrefix(clean, "../")
}

func validRevision(value string) bool {
	return len(value) == 40 && strings.Trim(value, "0123456789abcdef") == ""
}

func readOCIPlatforms(path string) ([]string, error) {
	rootData, err := readTarEntry(path, "index.json")
	if err != nil {
		return nil, err
	}
	var root struct {
		Manifests []struct {
			Digest string `json:"digest"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(rootData, &root); err != nil || len(root.Manifests) != 1 {
		return nil, errors.New("invalid OCI root index")
	}
	innerPath := "blobs/sha256/" + strings.TrimPrefix(root.Manifests[0].Digest, "sha256:")
	innerData, err := readTarEntry(path, innerPath)
	if err != nil {
		return nil, err
	}
	var inner struct {
		Manifests []struct {
			Platform struct {
				OS           string `json:"os"`
				Architecture string `json:"architecture"`
			} `json:"platform"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(innerData, &inner); err != nil {
		return nil, errors.New("invalid OCI platform index")
	}
	set := make(map[string]bool)
	for _, item := range inner.Manifests {
		if item.Platform.OS == "linux" && (item.Platform.Architecture == "amd64" || item.Platform.Architecture == "arm64") {
			set[item.Platform.OS+"/"+item.Platform.Architecture] = true
		}
	}
	platforms := make([]string, 0, len(set))
	for platform := range set {
		platforms = append(platforms, platform)
	}
	sort.Strings(platforms)
	return platforms, nil
}

func readTarEntry(path, name string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := tar.NewReader(file)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("OCI entry not found: %s", name)
		}
		if err != nil {
			return nil, err
		}
		if header.Name == name {
			if header.Size > 8<<20 {
				return nil, fmt.Errorf("OCI JSON entry is too large: %s", name)
			}
			return io.ReadAll(reader)
		}
	}
}

func commandOutput(name string, args ...string) (string, error) {
	command := exec.Command(name, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func writeExclusive(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create evidence: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var difference byte
	for i := range left {
		difference |= left[i] ^ right[i]
	}
	return difference == 0
}
