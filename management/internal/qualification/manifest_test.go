package qualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	adminui "github.com/chennqqi/rabbit-jetstream/admin-ui"
)

const testRevision = "0123456789abcdef0123456789abcdef01234567"

func TestLoadBindsRuntimeAndReturnsSafeReport(t *testing.T) {
	runtime := RuntimeIdentity{Version: "v0.1.0-test", Revision: testRevision, Clean: true, UIAssets: adminui.EmbeddedAssetIdentity()}
	path := writeManifest(t, runtime, nil)
	report, err := Load(path, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if report.Statement != "local-release-gates-passed; native-linux-soak-and-canary-required" || len(report.ManifestDigest) != 64 {
		t.Fatalf("unexpected report: %#v", report)
	}
	if strings.Contains(string(mustJSON(t, report)), path) {
		t.Fatal("safe report exposed local manifest path")
	}
}

func TestLoadRejectsUntrustedOrMismatchedManifest(t *testing.T) {
	base := RuntimeIdentity{Version: "v0.1.0-test", Revision: testRevision, Clean: true, UIAssets: adminui.EmbeddedAssetIdentity()}
	tests := []struct {
		name   string
		change func(*RuntimeIdentity, map[string]any)
	}{
		{"unclean runtime", func(r *RuntimeIdentity, _ map[string]any) { r.Clean = false }},
		{"runtime revision", func(r *RuntimeIdentity, _ map[string]any) { r.Revision = "manual" }},
		{"version mismatch", func(_ *RuntimeIdentity, m map[string]any) { m["version"] = "other" }},
		{"revision mismatch", func(_ *RuntimeIdentity, m map[string]any) { m["server_revision"] = strings.Repeat("a", 40) }},
		{"UI digest mismatch", func(_ *RuntimeIdentity, m map[string]any) {
			m["webui"].(map[string]any)["digest"] = strings.Repeat("a", 64)
		}},
		{"UI count mismatch", func(_ *RuntimeIdentity, m map[string]any) { m["webui"].(map[string]any)["file_count"] = 99 }},
		{"blank statement", func(_ *RuntimeIdentity, m map[string]any) { m["qualification"] = " " }},
		{"missing platforms", func(_ *RuntimeIdentity, m map[string]any) { delete(m, "platforms") }},
		{"invalid SDK revision", func(_ *RuntimeIdentity, m map[string]any) { m["sdk"].(map[string]any)["revision"] = "dirty" }},
		{"unsafe artifact", func(_ *RuntimeIdentity, m map[string]any) {
			m["artifacts"].([]any)[0].(map[string]any)["path"] = "../secret"
		}},
		{"unknown field", func(_ *RuntimeIdentity, m map[string]any) { m["unexpected"] = true }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runtime := base
			path := writeManifest(t, runtime, func(value map[string]any) { tc.change(&runtime, value) })
			if _, err := Load(path, runtime); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestLoadUnconfiguredIsUnreported(t *testing.T) {
	report, err := Load("", RuntimeIdentity{})
	if err != nil || report != nil {
		t.Fatalf("got %#v, %v", report, err)
	}
}

func TestLoadRejectsTrailingAndOversizedInputAndHashesBOM(t *testing.T) {
	runtime := RuntimeIdentity{Version: "v0.1.0-test", Revision: testRevision, Clean: true, UIAssets: adminui.EmbeddedAssetIdentity()}
	path := writeManifest(t, runtime, nil)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, []byte("{}")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path, runtime); err == nil {
		t.Fatal("trailing JSON was accepted")
	}
	if err := os.WriteFile(path, make([]byte, maxManifestSize+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path, runtime); err == nil {
		t.Fatal("oversized manifest was accepted")
	}
	bomRaw := append([]byte{0xef, 0xbb, 0xbf}, raw...)
	if err := os.WriteFile(path, bomRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := Load(path, runtime)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(bomRaw)
	if report.ManifestDigest != hex.EncodeToString(digest[:]) {
		t.Fatal("manifest digest did not bind the original BOM-prefixed bytes")
	}
}

func writeManifest(t *testing.T, runtime RuntimeIdentity, change func(map[string]any)) string {
	t.Helper()
	value := map[string]any{
		"schema": manifestSchema, "version": runtime.Version, "generated_at": "2026-01-01T00:00:00Z", "server_revision": runtime.Revision,
		"sdk": map[string]any{"version": "0.1.0-test", "revision": testRevision}, "contract_version": "v1", "nats_version": "v2.12.0",
		"webui":     map[string]any{"algorithm": runtime.UIAssets.Algorithm, "digest": runtime.UIAssets.Digest, "file_count": runtime.UIAssets.FileCount},
		"platforms": []string{"linux/amd64"}, "qualification": "local-release-gates-passed; native-linux-soak-and-canary-required",
		"artifacts": []any{map[string]any{"path": "bin/linux-amd64/rjs-management", "bytes": 1, "sha256": strings.Repeat("b", 64)}},
	}
	if change != nil {
		change(value)
	}
	raw := mustJSON(t, value)
	path := filepath.Join(t.TempDir(), "release-manifest.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
