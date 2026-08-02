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
		content := read(file)
		if !strings.Contains(content, "golang:1.25.12-alpine") {
			t.Errorf("%s is not pinned to the patched Go toolchain", file)
		}
		for lineNumber, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "FROM ") && !strings.Contains(line, "@sha256:") {
				t.Errorf("%s base image on line %d is not digest-pinned: %s", file, lineNumber+1, line)
			}
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
	if !strings.Contains(workflow, "tests/deployment/kubernetes-smoke.sh") {
		t.Error("CI no longer installs the chart in a real Kubernetes cluster")
	}
	kubernetesSmoke := read("tests/deployment/kubernetes-smoke.sh")
	for _, requirement := range []string{"sigs.k8s.io/kind@v0.31.0", "kindest/node:v1.35.0@sha256:", "load docker-image", "ctr -n k8s.io images tag", "rollout status", "sort -u", "get pvc", "^Bound$", "helm:3.18.4@sha256:", "busybox:1.37.0@sha256:", "auth_before", "auth_after", "network-allowed", "network-denied", "management NetworkPolicy allowed", "nats-network-allowed", "nats-network-denied", "NATS NetworkPolicy allowed", `"kind":"Eviction"`, "disruptionsAllowed", "disruption budget", "two simultaneous voluntary disruptions", " test ", "/readyz", "/admin/", " uninstall "} {
		if !strings.Contains(kubernetesSmoke, requirement) {
			t.Errorf("Kubernetes smoke gate lost requirement %q", requirement)
		}
	}
	kindConfig := read("tests/deployment/kind.yaml")
	if strings.Count(kindConfig, "role: worker") != 3 {
		t.Error("Kubernetes smoke gate must create exactly three worker nodes")
	}
	productionSmokeValues := read("tests/deployment/production-smoke-values.yaml")
	for _, requirement := range []string{"production:\n  enabled: true", "allowSameNamespace: false", "egress:\n    enabled: true", "rjs-nats-server-tls", "rjs-nats-client-tls", "rabbit-jetstream.io/nats-client"} {
		if !strings.Contains(productionSmokeValues, requirement) {
			t.Errorf("Kubernetes production smoke values lost requirement %q", requirement)
		}
	}
	tlsFixture := read("tests/helpers/tls-fixture/main.go")
	if !strings.Contains(tlsFixture, `filepath.Join(*output, "server")`) || !strings.Contains(tlsFixture, `filepath.Join(*output, "client")`) || !strings.Contains(kubernetesSmoke, "server/tls.key") || !strings.Contains(kubernetesSmoke, "client/tls.key") {
		t.Error("Kubernetes production smoke gate no longer proves distinct server and client TLS keys")
	}
	if !strings.Contains(workflow, "make verify-upstream-online") || !strings.Contains(read("Makefile"), "go run ./tools/upstreamcheck -online") {
		t.Error("CI no longer verifies the official NATS subtree provenance")
	}
	upstreamJobStart := strings.Index(workflow, "  upstream-build:")
	upstreamJobEnd := strings.Index(workflow, "\n  security:")
	if upstreamJobStart < 0 || upstreamJobEnd <= upstreamJobStart || !strings.Contains(workflow[upstreamJobStart:upstreamJobEnd], "fetch-depth: 0") {
		t.Error("CI upstream provenance job must fetch the historical subtree commit")
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
	for _, file := range []string{"deploy/compose/standalone.yml", "deploy/compose/cluster.yml"} {
		compose := read(file)
		for _, requirement := range []string{"restart: unless-stopped", "read_only: true", `security_opt: ["no-new-privileges:true"]`, `cap_drop: ["ALL"]`} {
			if !strings.Contains(compose, requirement) {
				t.Errorf("%s lost container hardening %q", file, requirement)
			}
		}
		if !strings.Contains(compose, "prom/prometheus:v3.12.0-distroless@sha256:") {
			t.Errorf("%s uses a mutable Prometheus image", file)
		}
	}
	healthHook := read("deploy/helm/rabbit-jetstream/templates/tests/health.yaml")
	if !strings.Contains(healthHook, "busybox:1.37.0@sha256:") {
		t.Error("Helm health hook uses a mutable image")
	}
	if !strings.Contains(healthHook, "hook-delete-policy: before-hook-creation") || strings.Contains(healthHook, "hook-succeeded") {
		t.Error("Helm health hook must retain successful Pod logs until the next test")
	}
	helmTest := read("tests/deployment/helm.ps1")
	for _, image := range []string{"alpine/helm:3.18.4@sha256:", "ghcr.io/yannh/kubeconform:v0.6.7@sha256:", "busybox:1.37.0@sha256:"} {
		if !strings.Contains(helmTest, image) {
			t.Errorf("Helm deployment test uses mutable helper %q", image)
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
	if !strings.Contains(notes, "production.enabled=true") || strings.Contains(notes, "immutable image tags") {
		t.Error("Helm production guidance must require digest-pinned images")
	}
	productionValidation := read("deploy/helm/rabbit-jetstream/templates/production-validation.yaml")
	for _, requirement := range []string{"nats.replicaCount >= 3", "management.replicaCount >= 2", "nats.storage.storageClass", "nats.tls.enabled=true", "distinct NATS server and client TLS Secrets", "nats.image.digest", "management.image.digest", "operator.image.digest", "networkPolicy.enabled=true", "networkPolicy.natsClients.allowSameNamespace", "networkPolicy.egress.enabled=true", "networkPolicy.egress.additionalRules", "podDisruptionBudget.enabled=true", "management.service.type=ClusterIP", "ingress.tls"} {
		if !strings.Contains(productionValidation, requirement) {
			t.Errorf("Helm production profile lost invariant %q", requirement)
		}
	}
	managementTemplate := read("deploy/helm/rabbit-jetstream/templates/management.yaml")
	if !strings.Contains(managementTemplate, "minDomains: {{ .Values.management.replicaCount }}") || !strings.Contains(managementTemplate, "DoNotSchedule") {
		t.Error("production management replicas no longer require distinct nodes")
	}
	networkPolicyTemplate := read("deploy/helm/rabbit-jetstream/templates/networkpolicy.yaml")
	for _, requirement := range []string{"serviceMonitor.enabled", "networkPolicy.monitoring.enabled", "networkPolicy.monitoring.namespaceSelector", "networkPolicy.monitoring.podSelector"} {
		if !strings.Contains(networkPolicyTemplate, requirement) {
			t.Errorf("ServiceMonitor NetworkPolicy integration lost requirement %q", requirement)
		}
	}
	secretTemplate := read("deploy/helm/rabbit-jetstream/templates/secret.yaml")
	for _, requirement := range []string{`hasKey $current.data "nats-password"`, `hasKey $current.data "nats-password-bcrypt"`, `$passwordHash = (index $current.data "nats-password-bcrypt" | b64dec)`, `hasKey $current.data "admin-token"`} {
		if !strings.Contains(secretTemplate, requirement) {
			t.Errorf("Helm Secret upgrade lost idempotency guard %q", requirement)
		}
	}
	release := read(".github/workflows/release.yml")
	releaseGateStart := strings.Index(release, "  release-gates:")
	releaseGateEnd := strings.Index(release, "\n  publish:")
	if releaseGateStart < 0 || releaseGateEnd <= releaseGateStart || !strings.Contains(release[releaseGateStart:releaseGateEnd], "fetch-depth: 0") {
		t.Error("release provenance gate must fetch the historical subtree commit")
	}
	for _, requirement := range []string{"needs: release-gates", "make verify-upstream-online", "./tests/security/scan.ps1", "./tests/deployment/helm.ps1", "tests/deployment/kubernetes-smoke.sh", "./tests/integration/rolling-upgrade.ps1", "./tests/integration/backup-restore.ps1", "-require-soak", "-source-revision", ".source_revision", "platforms: linux/amd64,linux/arm64", "alpine/helm:3.18.4@sha256:", "sbom: true", "provenance: mode=max", "push-to-registry: true", "SHA256SUMS"} {
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
