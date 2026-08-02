package main

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	platforms, err := readOCIPlatforms(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(platforms, ",") != "linux/amd64,linux/arm64" {
		t.Fatalf("platforms = %v", platforms)
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
