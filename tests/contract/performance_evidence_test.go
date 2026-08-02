package contract_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseSoakRetainsIndependentlyVerifiableEvidence(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	read := func(relative string) string {
		t.Helper()
		value, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		return string(value)
	}
	harness := read("tests/performance/jetstream.ps1")
	for _, required := range []string{"performance-evidence/v1alpha1", "Get-SHA256", "docker info", "git diff --quiet HEAD", "./tools/perfevidence", "-require-soak", "ConsumerStartDelay", "ConsumerDelay", "peak_backlog_messages", "drain_messages_per_second"} {
		if !strings.Contains(harness, required) {
			t.Errorf("performance harness lost release evidence requirement %q", required)
		}
	}
	makefile := read("Makefile")
	if !strings.Contains(makefile, "verify-soak:") || !strings.Contains(makefile, "-require-soak") {
		t.Error("Makefile no longer exposes independent soak evidence verification")
	}
	if !strings.Contains(makefile, "verify-native-bundle:") || !strings.Contains(makefile, "./tools/nativequal") {
		t.Error("Makefile no longer exposes native Linux bundle qualification")
	}
	nativeQualification := read("tools/nativequal/main.go")
	for _, required := range []string{"native-linux-preflight/v1alpha1", "WSL is not accepted", "Docker Desktop is not accepted", "SHA256SUMS", "linux/amd64,linux/arm64", "rjs-management", "--version"} {
		if !strings.Contains(nativeQualification, required) {
			t.Errorf("native Linux qualification lost requirement %q", required)
		}
	}
	comparison := read("tests/performance/native-sdk-compare.ps1")
	for _, required := range []string{"client-performance-comparison/v1alpha1", "rjs-sdk-bench", "publish_throughput", "consume_throughput", "publish_p99", "Assert-Integrity"} {
		if !strings.Contains(comparison, required) {
			t.Errorf("native SDK comparison lost requirement %q", required)
		}
	}
}
