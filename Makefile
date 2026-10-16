# golaw local pipeline. Every target is one plain command per line so that
# GNU make runs it unchanged on macOS, Linux and Windows runners: no shell
# feature, no awk, no tar. Remote CI runs make ci; nothing here is a gate on
# a developer's machine.

.PHONY: fmt lint test coverage build ci

fmt:
	golangci-lint fmt ./...

lint:
	golangci-lint run ./...

test:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...

coverage:
	go run ./tools/coverage -profile coverage.out -root . -min 80 -min-statements 20

build:
	go build ./...

ci: fmt lint test coverage build
