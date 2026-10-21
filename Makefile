# golaw local pipeline. Every target is one plain command per line so that
# GNU make runs it unchanged on macOS, Linux and Windows runners: no shell
# feature, no awk, no tar. Remote CI runs make ci; nothing here is a gate on
# a developer's machine.

.PHONY: mod deps-age fmt lint lint-pkg test coverage build scan secrets ci vet coverage-check generate-check deadcode licenses vuln

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

vet:
	go vet ./...

generate-check:
	go generate ./...
	git diff --exit-code

test:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...

coverage:
	go run ./cmd/golaw coverage --input coverage.out

coverage-check: coverage

build:
	go build ./...

deadcode:
	go mod tidy -diff

licenses:
	go run github.com/google/go-licenses/v2@v2.0.1 check --disallowed_types=forbidden,restricted ./...

vuln:
	go run ./tools/vulngate -scan -waivers golaw.yml

scan:
	go run ./cmd/golaw scan .

secrets:
	gitleaks dir --redact --no-banner .
	gitleaks git --redact --no-banner .

ci: mod deps-age fmt lint test coverage build scan secrets vet generate-check coverage-check deadcode licenses
