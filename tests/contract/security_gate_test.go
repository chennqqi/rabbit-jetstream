package contract_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseBuildsAndCIRetainSecurityGate(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	read := func(relative string) string {
		t.Helper()
		value, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		return string(value)
	}
	for _, file := range []string{"packaging/Dockerfile.nats-server", "packaging/Dockerfile.management", "packaging/Dockerfile.operator"} {
		if !strings.Contains(read(file), "golang:1.25.12-alpine") {
			t.Errorf("%s is not pinned to the patched Go toolchain", file)
		}
	}
	natsImage := read("packaging/Dockerfile.nats-server")
	if !strings.Contains(natsImage, "NATS_X_CRYPTO_VERSION=v0.52.0") {
		t.Error("NATS image lost its documented security dependency override")
	}
	operator := read("packaging/Dockerfile.operator")
	if strings.Contains(operator, "nats-box") || !strings.Contains(operator, "NATSCLI_VERSION=v0.4.0") || !strings.Contains(operator, "golang.org/x/net@v0.55.0") || !strings.Contains(operator, "-X main.version=${VERSION}") || !strings.Contains(operator, "GOARCH=$TARGETARCH") {
		t.Error("operator image no longer builds the minimal patched nats CLI")
	}
	scanner := read("tests/security/scan.ps1")
	if !strings.Contains(scanner, "govulncheck@v1.6.0") || !strings.Contains(scanner, "aquasec/trivy@sha256:") || !strings.Contains(scanner, "--severity 'HIGH,CRITICAL'") {
		t.Error("security scanner versions or severity gate are not pinned")
	}
	if !strings.Contains(read(".github/workflows/ci.yml"), "./tests/security/scan.ps1") {
		t.Error("CI no longer runs the repository security gate")
	}
	workflow := read(".github/workflows/ci.yml")
	if !strings.Contains(workflow, "scenario: [api, reconcile, apply, delete, audit, auth, routing, dlq, metrics, diagnostics, controller]") {
		t.Error("CI management scenario matrix is incomplete")
	}
	if !strings.Contains(workflow, "make verify-upstream-online") || !strings.Contains(read("Makefile"), "go run ./tools/upstreamcheck -online") {
		t.Error("CI no longer verifies the official NATS subtree provenance")
	}
	for _, workflowFile := range []string{".github/workflows/ci.yml", ".github/workflows/release.yml"} {
		for lineNumber, line := range strings.Split(read(workflowFile), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 3 || fields[0] != "-" || fields[1] != "uses:" {
				continue
			}
			separator := strings.LastIndexByte(fields[2], '@')
			if separator < 0 {
				t.Errorf("%s action on line %d has no immutable revision", workflowFile, lineNumber+1)
				continue
			}
			revision := fields[2][separator+1:]
			if len(revision) != 40 || strings.Trim(revision, "0123456789abcdef") != "" {
				t.Errorf("%s action on line %d is not pinned to a full commit SHA: %s", workflowFile, lineNumber+1, fields[2])
			}
		}
	}
	for _, file := range []string{
		"tests/integration/docker-desktop.ps1",
		"tests/integration/linux-smoke.sh",
		"tests/integration/rolling-upgrade.ps1",
		"tests/integration/shadow-capture.ps1",
		"tests/fault/single-node-recovery.sh",
		"tests/performance/jetstream.ps1",
	} {
		content := read(file)
		if strings.Contains(content, "natsio/nats-box:latest") || strings.Contains(content, "golang:1.25-bookworm") {
			t.Errorf("%s uses a floating test-helper image", file)
		}
	}
	for _, file := range []string{"tests/coverage/check.sh", "tests/coverage/check.ps1"} {
		content := read(file)
		if !strings.Contains(content, "internal/topology") || !strings.Contains(content, "management/internal/controller") || !strings.Contains(content, "90.0") {
			t.Errorf("%s does not enforce the critical-package coverage gate", file)
		}
	}
	schema := read("deploy/helm/rabbit-jetstream/values.schema.json")
	helpers := read("deploy/helm/rabbit-jetstream/templates/_helpers.tpl")
	if !strings.Contains(schema, `"digest": {"type": "string", "pattern": "^(|sha256:[a-f0-9]{64})$"}`) || !strings.Contains(helpers, `printf "%s@%s"`) {
		t.Error("Helm chart no longer validates and renders immutable image digests")
	}
	notes := read("deploy/helm/rabbit-jetstream/templates/NOTES.txt")
	if !strings.Contains(notes, "pin published images by digest") || strings.Contains(notes, "immutable image tags") {
		t.Error("Helm production guidance must require digest-pinned images")
	}
	release := read(".github/workflows/release.yml")
	for _, requirement := range []string{"needs: release-gates", "make verify-upstream-online", "./tests/security/scan.ps1", "./tests/deployment/helm.ps1", "./tests/integration/rolling-upgrade.ps1", "./tests/integration/backup-restore.ps1", "-require-soak", ".source_revision", "platforms: linux/amd64,linux/arm64", "sbom: true", "provenance: mode=max", "push-to-registry: true", "SHA256SUMS"} {
		if !strings.Contains(release, requirement) {
			t.Errorf("release workflow lost requirement %q", requirement)
		}
	}
	for _, operatorMetadata := range []string{"OPERATOR_IMAGE:", "OPERATOR_DIGEST:", `operator:\n  image:`, `"$OPERATOR_IMAGE" "$OPERATOR_DIGEST"`} {
		if !strings.Contains(release, operatorMetadata) {
			t.Errorf("release image metadata lost operator field %q", operatorMetadata)
		}
	}
}
