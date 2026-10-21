package assets_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/runner"
)

func TestMakeAssetsKeepLongChecksExplicit(t *testing.T) {
	t.Parallel()
	assertAsset(t, "Makefile", "include golaw.mk\n", assets.OwnedMerged)
	got := renderedAsset(t, "golaw.mk", assetContext())
	if got.Template.Ownership != assets.OwnedByKit {
		t.Fatal("build include ownership is not kit")
	}
	body := string(got.Data)
	if !strings.HasPrefix(body, "export GOTOOLCHAIN=local\n") {
		t.Fatal("the module toolchain is not local by default")
	}
	for _, want := range []string{
		"\nPKG ?= .\nFUZZ ?= Fuzz\nFUZZTIME ?= 30s\n",
		"\n.PHONY: mod fmt lint vet test coverage-check build scan secrets deadcode bench fuzz test-integration ",
		"\nmod:\n\tgo mod verify\n",
		"\nfmt:\n\tgolaw lock pre-commit -- golangci-lint fmt ./...\n",
		"\nlint:\n\tgolangci-lint run ./...\n",
		"\nvet:\n\tgo vet ./...\n",
		"\ntest:\n\tgo test -race -shuffle=on -count=1 -timeout=10m -coverpkg=./... -coverprofile=coverage.out -covermode=atomic ./...\n",
		"\ncoverage-check:\n\tgolaw coverage --input coverage.out\n",
		"\nbuild:\n\tgo build ./...\n",
		"\nscan:\n\tgolaw scan $(if $(GOLAW_ENROLLED),--enrolled $(GOLAW_ENROLLED)) .\n",
		"\nsecrets:\n\tgitleaks dir --redact --no-banner .\n\tgitleaks git --redact --no-banner .\n",
		"\ndeadcode:\n\tgo test -tags=deadcode ./test/deadcode/\n",
		"\nbench:\n\tgo test -run=^$$ -bench=. -benchmem ./...\n",
		"\nfuzz:\n\tgo test -run=^$$ -fuzz=$(FUZZ) -fuzztime=$(FUZZTIME) $(PKG)\n",
		"\ntest-integration:\n\tgo test -race -tags=integration ./...\n",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing retained build contract: %q", want)
		}
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
