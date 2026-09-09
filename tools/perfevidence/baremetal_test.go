package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBareMetalEvidence(t *testing.T) {
	for _, fault := range []string{"", "image", "digest", "preflight-revision", "restart", "unhealthy", "identity"} {
		t.Run("fault="+fault, func(t *testing.T) {
			dir := t.TempDir()
			r := validReport()
			reportPath := filepath.Join(dir, "report.json")
			resourcePath := filepath.Join(dir, "resources.ndjson")
			prePath := filepath.Join(dir, "preflight.json")
			proofPath := filepath.Join(dir, "proof.json")
			writeJSON(t, reportPath, r)
			pre := nativePreflight{Schema: "rabbit-jetstream.io/native-linux-preflight/v1alpha1", SourceRevision: testRevision, Runtime: "linux/amd64", DeploymentMode: "bare-metal", NATSBinarySHA256: strings.Repeat("a", 64), VerificationRevision: strings.Repeat("b", 40), BundleManifestSHA256: strings.Repeat("c", 64), Host: map[string]any{"os_type": "linux", "architecture": "amd64", "operating_system": "Oracle Linux", "kernel_version": "6.12", "cpus": 48, "memory_bytes": 256 << 30}}
			writeJSON(t, prePath, pre)
			var samples strings.Builder
			for h := 0; h <= 24; h++ {
				for n := 1; n <= 3; n++ {
					id := fmt.Sprintf("id-%d", n)
					healthy := true
					start := "2026-01-01T00:00:00Z"
					if h == 12 && n == 1 {
						switch fault {
						case "restart":
							id = "new"
						case "unhealthy":
							healthy = false
						case "identity":
							start = ""
						}
					}
					v := map[string]any{"captured_at": r.StartedAt.Add(time.Duration(h) * time.Hour), "node": fmt.Sprintf("nats-%d", n), "healthy": healthy, "server_statistics": map[string]any{"server_id": id, "start": start}}
					raw, _ := json.Marshal(v)
					samples.Write(raw)
					samples.WriteByte('\n')
				}
			}
			if err := os.WriteFile(resourcePath, []byte(samples.String()), 0600); err != nil {
				t.Fatal(err)
			}
			opt := generateOptions{EvidencePath: proofPath, ReportPath: reportPath, ResourcesPath: resourcePath, PreflightPath: prePath, NATSBinarySHA256: pre.NATSBinarySHA256, CommandJSON: `["jetstream-bench","-duration","24h"]`, SampleInterval: 3600, MinPublish: 900, MinConsume: 900, MaxP99: 10}
			if fault == "image" {
				opt.NATSImageID = strings.Repeat("d", 64)
			}
			if fault == "digest" {
				opt.NATSBinarySHA256 = strings.Repeat("d", 64)
			}
			_, err := createInauguralEvidence(opt)
			if fault == "image" || fault == "digest" {
				if err == nil {
					t.Fatal("mixed/mismatched provenance accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if fault == "preflight-revision" {
				pre.SourceRevision = strings.Repeat("d", 40)
				writeJSON(t, prePath, pre)
			}
			err = verifyEvidence(proofPath, true, testRevision)
			if fault == "" && err != nil {
				t.Fatal(err)
			}
			if fault != "" && err == nil {
				t.Fatal("invalid bare-metal evidence accepted")
			}
		})
	}
}
