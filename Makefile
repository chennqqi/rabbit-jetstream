.PHONY: build build-upstream build-operator test test-race test-linux-smoke test-linux-fault test-helm coverage coverage-check fmt run

build:
	go build -o bin/rjs-management ./management/cmd/rjs-management
	go build -o bin/rjsctl ./tools/rjsctl

build-upstream:
	cd upstream/nats-server && go build -o ../../bin/nats-server .

build-operator:
	docker build -f packaging/Dockerfile.operator -t rabbit-jetstream/operator:local .

test:
	go test ./...

test-race:
	go test -race ./...

test-linux-smoke:
	bash tests/integration/linux-smoke.sh

test-linux-fault:
	bash tests/fault/single-node-recovery.sh

test-helm:
	pwsh -File tests/deployment/helm.ps1

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

coverage-check:
	bash tests/coverage/check.sh

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './outlink/*')

run:
	go run ./management/cmd/rjs-management
