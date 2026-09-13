.PHONY: build build-admin-ui generate-openapi verify-openapi-generated verify-admin-ui-dist build-upstream build-operator verify-upstream verify-upstream-online test test-race test-admin-ui test-admin-ui-cluster test-admin-ui-all test-linux-smoke test-linux-fault test-performance test-scale test-consumer-scale test-soak verify-soak verify-native-bundle verify-release-approval test-helm test-kubernetes test-rolling test-security test-local-rc test-local-release package-local-rc coverage coverage-check fmt run

BUNDLE ?= dist/v0.1.0-rc.2
NATIVE_QUAL_OUTPUT ?= native-linux-preflight.json
RELEASE_APPROVAL ?= release-approval.json

build:
	go build -o bin/rjs-management ./management/cmd/rjs-management
	go build -o bin/rjsctl ./tools/rjsctl

build-admin-ui:
	cd admin-ui && npm ci --ignore-scripts && npm test && npm run build && npm run promote

generate-openapi:
	pwsh -NoProfile -File scripts/openapi-codegen.ps1

verify-openapi-generated:
	pwsh -NoProfile -File scripts/openapi-codegen.ps1 -Check

verify-admin-ui-dist:
	cd admin-ui && npm run verify:dist

build-upstream:
	cd upstream/nats-server && go build -o ../../bin/nats-server .

verify-upstream:
	go run ./tools/upstreamcheck

verify-upstream-online:
	go run ./tools/upstreamcheck -online

build-operator:
	docker build -f packaging/Dockerfile.operator -t rabbit-jetstream/operator:local .

test:
	go test ./...

test-race:
	go test -race ./...

test-admin-ui:
	pwsh -NoProfile -File tests/admin-ui/run.ps1

test-admin-ui-cluster:
	pwsh -NoProfile -File tests/admin-ui/run.ps1 -Project rjs-admin-ui-cluster-e2e -DeploymentProfile cluster

test-admin-ui-all: test-admin-ui test-admin-ui-cluster

test-linux-smoke:
	bash tests/integration/linux-smoke.sh

test-linux-fault:
	bash tests/fault/single-node-recovery.sh

test-performance:
	pwsh -NoProfile -File tests/performance/jetstream.ps1 -Mode ci

test-scale:
	pwsh -NoProfile -File tests/performance/jetstream.ps1 -Mode scale -Output performance-scale.json

test-consumer-scale:
	go test ./management/internal/api -run '^$$' -bench 'Benchmark(Collect|Query|HTTP)GlobalConsumers100k$$' -benchtime=1x -benchmem -count=5

test-soak:
	pwsh -NoProfile -File tests/performance/jetstream.ps1 -Mode soak -Output performance-soak.json -Baseline performance-baseline.json

verify-soak:
	go run ./tools/perfevidence -evidence performance-soak.json.evidence.json -require-soak -source-revision "$$(git rev-parse HEAD)"

verify-native-bundle:
	go run ./tools/nativequal -bundle "$(BUNDLE)" -output "$(NATIVE_QUAL_OUTPUT)" -source-revision "$$(git rev-parse HEAD)"

verify-release-approval:
	go run ./tools/releaseapproval -evidence "$(RELEASE_APPROVAL)" -source-revision "$$(git rev-parse HEAD)"

test-helm:
	pwsh -File tests/deployment/helm.ps1

test-kubernetes:
	tests/deployment/kubernetes-smoke.sh

test-rolling:
	pwsh -File tests/integration/rolling-upgrade.ps1 -BuildLocal

test-security:
	pwsh -NoProfile -File tests/security/scan.ps1

test-local-rc:
	pwsh -NoProfile -File scripts/release/local-rc.ps1 -Mode Quick

test-local-release:
	pwsh -NoProfile -File scripts/release/local-rc.ps1 -Mode Release

package-local-rc:
	pwsh -NoProfile -File scripts/release/package-local.ps1 -Version v0.1.0-rc.2

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

coverage-check:
	bash tests/coverage/check.sh

fmt:
	gofmt -w $$(find admin-ui api internal management tests tools -name '*.go')

run:
	go run ./management/cmd/rjs-management
