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
	for _, required := range []string{"performance-evidence/v1alpha1", "Get-SHA256", "docker info", "git diff --quiet HEAD", "./tools/perfevidence", "-require-soak"} {
		if !strings.Contains(harness, required) {
			t.Errorf("performance harness lost release evidence requirement %q", required)
		}
	}
	makefile := read("Makefile")
	if !strings.Contains(makefile, "verify-soak:") || !strings.Contains(makefile, "-require-soak") {
		t.Error("Makefile no longer exposes independent soak evidence verification")
	}
}
