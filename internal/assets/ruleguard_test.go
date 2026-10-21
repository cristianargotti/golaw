package assets_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
)

// dslRequire is the line golaw init proposes for go.mod, at the version
// the kit itself requires.
const dslRequire = "require github.com/quasilyte/go-ruleguard/dsl v0.3.23\n"

func TestRuleguardAssetIsTheKitRulesFile(t *testing.T) {
	t.Parallel()
	got := renderedAsset(t, "scripts/lint/rules.go", assetContext())
	want, err := os.ReadFile("../../scripts/lint/rules.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Data, want) || got.Template.Ownership != assets.OwnedByKit || len(got.Template.When) != 0 {
		t.Fatalf("the rules asset differs from the kit's scripts/lint/rules.go or is not a core kit file: %+v", got.Template)
	}
}

func TestRuleguardProposesTheDSLOnlyToAModuleWithoutIt(t *testing.T) {
	t.Parallel()
	kit, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(kit), "\t"+strings.TrimPrefix(dslRequire, "require ")) {
		t.Fatalf("the proposal %q is not the version the kit's go.mod requires", dslRequire)
	}
	without := renderedAsset(t, "go.mod", assetContext())
	if string(without.Data) != dslRequire || without.Template.Ownership != assets.OwnedMerged {
		t.Fatalf("go.mod proposal = %q, ownership %q", without.Data, without.Template.Ownership)
	}
	context := assetContext()
	context.DirectRequires = []assets.Requirement{{Path: "github.com/quasilyte/go-ruleguard/dsl", Version: "v0.3.23"}}
	if with := renderedAsset(t, "go.mod", context); len(with.Data) != 0 {
		t.Fatalf("a module that requires the dsl gets the proposal %q", with.Data)
	}
}

func TestInheritedBaselineRecordsTheDSLOnce(t *testing.T) {
	t.Parallel()
	const tools = "  - github.com/evilmartians/lefthook/v2\n  - github.com/golangci/golangci-lint/v2\n" +
		"  - github.com/zricethezav/gitleaks/v8\n  - golang.org/x/vuln\n"
	row := "| github.com/quasilyte/go-ruleguard/dsl | v0.3.23 | kit |"
	without := string(renderedAsset(t, "docs/adr/0001-inherited-baseline.md", assetContext()).Data)
	if !strings.HasPrefix(without, "---\n") || !strings.Contains(without, "modules:\n  - github.com/quasilyte/go-ruleguard/dsl\n"+tools+"---\n") ||
		!strings.Contains(without, "| --- | --- | --- |\n"+row+"\n") {
		t.Fatalf("a module without the dsl:\n%s", without)
	}
	context := assetContext()
	context.DirectRequires = []assets.Requirement{
		{Path: "example.test/library", Version: "v1.2.3"},
		{Path: "github.com/quasilyte/go-ruleguard/dsl", Version: "v0.3.23"},
	}
	with := string(renderedAsset(t, "docs/adr/0001-inherited-baseline.md", context).Data)
	if strings.Count(with, "github.com/quasilyte/go-ruleguard/dsl") != 2 || strings.Contains(with, row) ||
		!strings.Contains(with, "modules:\n  - example.test/library\n  - github.com/quasilyte/go-ruleguard/dsl\n"+tools+"---\n") {
		t.Fatalf("a module that requires the dsl:\n%s", with)
	}
}
