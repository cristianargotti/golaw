# golaw local pipeline. Every target is one plain command per line so that
# GNU make runs it unchanged on macOS, Linux and Windows runners: no shell
# feature, no awk, no tar. Remote CI runs make ci; nothing here is a gate on
# a developer's machine.

.PHONY: mod deps-age fmt lint lint-pkg test coverage build scan secrets ci

mod:
	go mod verify

deps-age:
	go run ./tools/depsage -days 7

fmt:
	golangci-lint fmt ./...

lint:
	golangci-lint run ./...

# One package, for a change that touched only it: make lint-pkg PKG=./internal/model/...
lint-pkg:
	golangci-lint run $(PKG)

test:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...

coverage:
	go run ./tools/coverage -profile coverage.out -root . -min 80 -min-statements 20

build:
	go build ./...

scan:
	go run ./cmd/golaw scan .

secrets:
	gitleaks dir --redact --no-banner .
	gitleaks git --redact --no-banner .

ci: mod deps-age fmt lint test coverage build scan secrets
