.PHONY: build test test-race coverage fmt run

build:
	go build -o bin/rjs-management ./management/cmd/rjs-management
	go build -o bin/rjsctl ./tools/rjsctl

test:
	go test ./...

test-race:
	go test -race ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './outlink/*')

run:
	go run ./management/cmd/rjs-management
