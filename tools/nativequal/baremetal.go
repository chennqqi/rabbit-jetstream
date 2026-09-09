package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func qualifyBareMetal(bundle, revision, dataPath string, env qualificationEnvironment) (evidence, error) {
	var result evidence
	if env.goos != "linux" || env.goarch != "amd64" || !validRevision(revision) {
		return result, errors.New("bare-metal qualification requires native linux/amd64 and a valid revision")
	}
	for _, marker := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := env.readFile(marker); err == nil {
			return result, errors.New("bare-metal mode must not run inside a container")
		} else if !errors.Is(err, os.ErrNotExist) {
			return result, err
		}
	}
	kernel, err := env.readFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return result, err
	}
	version, err := env.readFile("/proc/version")
	if err != nil {
		return result, err
	}
	if strings.Contains(strings.ToLower(string(kernel)+string(version)), "microsoft") {
		return result, errors.New("WSL is not accepted")
	}
	uid, err := env.command("id", "-u")
	if err != nil {
		return result, err
	}
	numericUID, err := strconv.Atoi(strings.TrimSpace(uid))
	if err != nil || numericUID <= 0 {
		return result, errors.New("bare-metal qualification must run as an unprivileged user")
	}
	info, err := os.Stat(dataPath)
	if err != nil || !info.IsDir() {
		return result, errors.New("bare-metal data path must be an existing directory")
	}
	filesystem, err := env.command("findmnt", "-n", "-o", "FSTYPE,SOURCE", "-T", dataPath)
	if err != nil {
		return result, err
	}
	mount := strings.Fields(filesystem)
	if len(mount) != 2 || (mount[0] != "xfs" && mount[0] != "ext4") {
		return result, errors.New("bare-metal evidence requires an identified persistent xfs/ext4 filesystem")
	}
	mem, err := env.readFile("/proc/meminfo")
	if err != nil {
		return result, err
	}
	memory, err := memTotal(mem)
	if err != nil {
		return result, err
	}
	cpu, err := env.readFile("/proc/stat")
	if err != nil {
		return result, err
	}
	cpus := 0
	for _, line := range strings.Split(string(cpu), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && strings.HasPrefix(fields[0], "cpu") && len(fields[0]) > 3 {
			if _, err := strconv.Atoi(fields[0][3:]); err == nil {
				cpus++
			}
		}
	}
	if cpus == 0 {
		return result, errors.New("no host CPUs found")
	}
	osRelease, err := env.readFile("/etc/os-release")
	if err != nil {
		return result, err
	}
	osName := ""
	for _, line := range strings.Split(string(osRelease), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			osName = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
		}
	}
	if osName == "" {
		return result, errors.New("OS identity missing")
	}
	manifest, err := readManifest(filepath.Join(bundle, "release-manifest.json"))
	if err != nil {
		return result, err
	}
	if manifest.DeploymentMode != "bare-metal" || manifest.ServerRevision != revision || !validRevision(manifest.VerificationRevision) || !validRevision(manifest.SDK.Revision) || !strings.Contains(manifest.Qualification, "native-linux-soak-and-canary-required") {
		return result, errors.New("bare-metal manifest identity or qualification boundary is invalid")
	}
	required := map[string]bool{}
	for _, name := range []string{"nats-server", "rjs-management", "rjsctl", "nativequal", "jetstream-bench", "resource-sampler", "perfevidence", "resourceaudit", "baremetal-run"} {
		required["bin/linux-amd64/"+name] = true
	}
	for _, path := range []string{"licenses/NATS-LICENSE", "licenses/release-license-records.md", "licenses/release-license-records.zh-CN.md"} {
		required[path] = true
	}
	seen := map[string]bool{}
	var natsDigest string
	for _, item := range manifest.Artifacts {
		if !safeRelativePath(item.Path) || seen[item.Path] {
			return result, errors.New("unsafe or duplicate bare-metal artifact")
		}
		seen[item.Path] = true
		actual, err := regularFileDigest(bundle, item.Path)
		if err != nil {
			return result, err
		}
		if actual != item.SHA256 {
			return result, fmt.Errorf("bare-metal artifact checksum mismatch: %s", item.Path)
		}
		if item.Path == "bin/linux-amd64/nats-server" {
			natsDigest = actual
		}
	}
	for path := range required {
		if !seen[path] {
			return result, fmt.Errorf("required bare-metal artifact missing: %s", path)
		}
	}
	count, err := verifyChecksums(bundle)
	if err != nil {
		return result, err
	}
	if count != len(manifest.Artifacts)+1 {
		return result, errors.New("bare-metal checksum coverage mismatch")
	}
	binaries := map[string]string{}
	for name, args := range map[string][]string{"rjsctl": {"version"}, "rjs-management": {"--version"}, "nats-server": {"--version"}} {
		value, err := env.command(filepath.Join(bundle, "bin/linux-amd64", name), args...)
		if err != nil {
			return result, err
		}
		expected := manifest.Version
		if name == "nats-server" {
			expected = "nats-server: " + manifest.NATSVersion
		}
		if value != expected {
			return result, fmt.Errorf("%s version mismatch: %q", name, value)
		}
		binaries[name] = value
	}
	manifestHash, err := regularFileDigest(bundle, "release-manifest.json")
	if err != nil {
		return result, err
	}
	return evidence{Schema: evidenceSchema, GeneratedAt: env.now().UTC().Format(time.RFC3339Nano), SourceRevision: revision, ReleaseVersion: manifest.Version,
		SDKVersion: manifest.SDK.Version, SDKRevision: manifest.SDK.Revision, ContractVersion: manifest.ContractVersion, NATSVersion: manifest.NATSVersion,
		Runtime: "linux/amd64", KernelRelease: strings.TrimSpace(string(kernel)), DeploymentMode: "bare-metal", NATSBinarySHA256: natsDigest,
		VerificationRevision: manifest.VerificationRevision, BundleManifestSHA256: manifestHash, ChecksumFiles: count, Binaries: binaries,
		Host: map[string]any{"os_type": "linux", "architecture": "amd64", "operating_system": osName, "kernel_version": strings.TrimSpace(string(kernel)), "cpus": cpus, "memory_bytes": memory, "filesystem": mount[0], "disk_source": mount[1], "data_path": dataPath, "uid": numericUID}}, nil
}

func memTotal(data []byte) (int64, error) {
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) == 3 && f[0] == "MemTotal:" && f[2] == "kB" {
			n, err := strconv.ParseInt(f[1], 10, 64)
			if err == nil && n > 0 && n < 1<<50 {
				return n * 1024, nil
			}
		}
	}
	return 0, errors.New("invalid host MemTotal")
}

func regularFileDigest(base, relative string) (string, error) {
	if !safeRelativePath(relative) {
		return "", errors.New("unsafe artifact path")
	}
	path := base
	for _, part := range strings.Split(relative, "/") {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("symlink artifacts are not accepted")
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("artifact is not a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
