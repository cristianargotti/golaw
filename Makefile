# golaw local pipeline. Every target is one plain command per line so that
# GNU make runs it unchanged on macOS, Linux and Windows runners: no shell
# feature, no awk, no tar. Remote CI runs make ci; nothing here is a gate on
# a developer's machine.

# make runs the toolchain the machine holds, never one go downloads by
# itself; the CI file installs the toolchain go.mod names before make ci.
export GOTOOLCHAIN=local

.PHONY: mod deps-age fmt lint lint-pkg test coverage build scan lint-surface coverage-new secrets ci vet coverage-check generate-check deadcode licenses vuln release-gate-dry-run release-verify

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

# The slowest package under -race outlasts go test's default of ten minutes
# on a loaded or slow machine; the CI job gives each system its measured
# make ci plus these 25 minutes, so a hung test meets this deadline first.
test:
	go test -race -timeout=25m -coverprofile=coverage.out -covermode=atomic ./...

coverage:
	go run ./cmd/golaw coverage --input coverage.out

coverage-check: coverage

build:
	go build ./...

deadcode:
	go test -tags=deadcode ./test/deadcode/

licenses:
	go run github.com/google/go-licenses/v2@v2.0.1 check --disallowed_types=forbidden,restricted ./...

vuln:
	go run ./tools/vulngate -scan -waivers golaw.yml

scan:
	go run ./cmd/golaw scan .

lint-surface:
	go run ./cmd/golaw lint --surface .

COVERAGE_BASE ?= origin/main

coverage-new: test
	go run ./cmd/golaw coverage --new-code --input coverage.out --base $(COVERAGE_BASE)

secrets:
	gitleaks dir --redact --no-banner .
	gitleaks git --redact --no-banner .

ci: mod deps-age fmt lint test coverage build scan secrets vet generate-check coverage-check deadcode licenses coverage-new lint-surface

# The release gate's decision replayed over a saved answer of the runs
# endpoint of ci.yml, on any machine: make release-gate-dry-run SHA=<commit> RUNS=<file>
release-gate-dry-run:
	go run ./tools/releasegate -sha $(SHA) -runs $(RUNS)

# A downloaded release verified by the downloaded binary itself, the only
# build doctor verifies: make release-verify BIN=<binary> DIST=<directory>
release-verify:
	$(BIN) doctor --release $(DIST)
