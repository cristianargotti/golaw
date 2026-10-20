package assets_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/runner"
)

func TestMakeAssetsKeepLongChecksExplicit(t *testing.T) {
	t.Parallel()
	const makeBody = `export GOTOOLCHAIN=local

PKG ?= .
FUZZ ?= Fuzz
FUZZTIME ?= 30s

.PHONY: mod fmt lint vet test coverage-check build scan secrets deadcode bench fuzz test-integration

mod:
	go mod verify

fmt:
	golaw lock format -- golangci-lint fmt ./...

lint:
	golangci-lint run ./...

vet:
	go vet ./...

test:
	go test -race -shuffle=on -count=1 -timeout=10m -coverpkg=./... -coverprofile=coverage.out -covermode=atomic ./...

coverage-check:
	golaw coverage --input coverage.out

build:
	go build ./...

scan:
	golaw scan .

secrets:
	gitleaks dir --redact --no-banner .
	gitleaks git --redact --no-banner .

deadcode:
	go test -tags=deadcode ./test/deadcode/

bench:
	go test -run=^$$ -bench=. -benchmem ./...

fuzz:
	go test -run=^$$ -fuzz=$(FUZZ) -fuzztime=$(FUZZTIME) $(PKG)

test-integration:
	go test -race -tags=integration ./...
`
	for _, tc := range []struct {
		path, want string
		ownership  assets.Ownership
	}{
		{"Makefile", "include golaw.mk\n", assets.OwnedMerged},
		{"golaw.mk", makeBody, assets.OwnedByKit},
	} {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			assertAsset(t, tc.path, tc.want, tc.ownership)
		})
	}
}

func TestToolsModuleCoversTheGoLineOfEveryPin(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ adopter, want string }{
		{"1.24.0", "1.26.6"}, {"1.26.6", "1.26.6"}, {"1.27.0", "1.27.0"},
	} {
		t.Run(tc.adopter, func(t *testing.T) {
			t.Parallel()
			context := assetContext()
			context.GoVersion = tc.adopter
			got := renderedAsset(t, "tools/go.mod", context)
			requireToolsModule(t, string(got.Data), tc.want)
			if got.Template.Ownership != assets.OwnedByKit {
				t.Fatal("tools module ownership is not kit")
			}
		})
	}
}

// requireToolsModule requires the rendered tools module to declare its
// module, the go line it covers, and a tool directive and the pin of every
// tool of the toolchain.
func requireToolsModule(t *testing.T, text, goLine string) {
	t.Helper()
	for _, want := range []string{
		"module example.test/service/tools\n",
		"go " + goLine + "\n",
		"tool (\n",
		"require (\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("tools module lacks %q", want)
		}
	}
	for _, tool := range runner.Toolchain() {
		if !strings.Contains(text, "\t"+tool.Module+"\n") {
			t.Errorf("missing tool directive: %s", tool.Module)
		}
		if !strings.Contains(text, " v"+tool.Version+"\n") {
			t.Errorf("missing pin: %s", tool.Name)
		}
	}
}
