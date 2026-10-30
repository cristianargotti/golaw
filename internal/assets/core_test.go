package assets_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestCompleteCoreInventoryHasNoProjectAgentFiles(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ main, decisions string }{
		{"root", "docs/adr"},
		{"cmd", "docs/decisions"},
	} {
		t.Run(tc.main, func(t *testing.T) {
			t.Parallel()
			context := assetContext()
			context.Main, context.Decisions = tc.main, tc.decisions
			files, err := assets.RenderAllContext(golaw.FS, context)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{
				".editorconfig", ".gitattributes", ".github/dependabot.yml",
				ciAssetPath,
				".gitleaks.toml", ".golangci.yml", ".gremlins.yaml",
				".mockery.yaml", "Makefile",
				tc.decisions + "/0001-inherited-baseline.md",
				"go.mod", "golaw.mk", "lefthook.yml", "scripts/lint/rules.go",
				"test/deadcode/reachability_test.go", "tools/go.mod",
			}
			slices.Sort(want)
			paths := make([]string, 0, len(files))
			for i := range files {
				paths = append(paths, files[i].Path)
				requireCoreFile(t, &files[i])
			}
			if !slices.Equal(paths, want) {
				t.Fatalf("core paths = %v, want %v", paths, want)
			}
		})
	}
}

// requireCoreFile requires a rendered core file to hold no expression and
// to land outside every project agent directory.
func requireCoreFile(t *testing.T, file *assets.Rendered) {
	t.Helper()
	body := string(file.Data)
	// The pipeline resolves the enrollment and runner expressions, so the CI
	// file keeps them.
	if file.Path == ciAssetPath {
		body = strings.ReplaceAll(body, ciEnrollmentExpression, "")
		body = strings.ReplaceAll(body, ciRunnerExpression, "")
	}
	if strings.Contains(body, "{{") {
		t.Errorf("remaining expression in %s", file.Path)
	}
	for _, forbidden := range []string{
		"." + "clau" + "de/", "." + "cur" + "sor/",
		"." + "co" + "dex/", "AGENTS.md", "CLA" + "UDE.md",
	} {
		if file.Path == forbidden || strings.HasPrefix(file.Path, forbidden) {
			t.Errorf("project agent destination: %s", file.Path)
		}
	}
}

// TestCompleteCoreTemplatesScanClean dispatches the production catalog over
// every core template as init renders it beside a module, so the first scan
// after init --apply, and the staged scan of the commit that adopts the
// files, never judge a file the kit wrote.
func TestCompleteCoreTemplatesScanClean(t *testing.T) {
	t.Parallel()
	registry, err := checks.Registry()
	if err != nil {
		t.Fatal(err)
	}
	for _, capabilities := range [][]string{{"go"}, model.Capabilities()} {
		t.Run(strings.Join(capabilities, ","), func(t *testing.T) {
			t.Parallel()
			tree := proof.NewTree(renderedRepository(t, capabilities))
			run, err := engine.Dispatch(t.Context(), registry, model.Input{
				Mode: model.ModeScan, Policy: initPolicy(t, capabilities), Snapshot: tree, Targets: tree.Files(),
			})
			if err != nil {
				t.Fatal(err)
			}
			decision := engine.Decide(run)
			for _, finding := range decision.Findings {
				t.Errorf("%s:%d %s %s", finding.Path, finding.Line, finding.Check, finding.Message)
			}
			for _, left := range decision.Unmeasured {
				t.Errorf("unmeasured: %+v", left)
			}
		})
	}
}

// dslSums are the go.sum lines of the ruleguard dsl the rendered go.mod
// requires once the person applies the proposal of init.
const dslSums = "github.com/quasilyte/go-ruleguard/dsl v0.3.23 h1:lxjt5B6ZCiBeeNO8/oQsegE6fLeCzuMRoVWSkXC4uvY=\n" +
	"github.com/quasilyte/go-ruleguard/dsl v0.3.23/go.mod h1:KeCP03KrjuSO0H1kTuZQCWlQPulDV6YMIXmpQss17rU=\n"

// initPolicy is the effective policy of the golaw.yml init writes beside
// the templates, so the scan judges the rendered files under the floors and
// the allowed lint exclusions init seeds.
func initPolicy(t *testing.T, capabilities []string) *model.EffectivePolicy {
	t.Helper()
	data, err := bootstrap.Render(bootstrap.Inputs{
		KitVersion: templateValues().KitVersion, Language: model.DefaultLanguage, Capabilities: capabilities,
		Layout: bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/adr", Go: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	file, err := policy.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := policy.Compile(file, time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC), nil)
	if err != nil {
		t.Fatal(err)
	}
	return compiled
}

// renderedRepository is every core template rendered for the
// capabilities, beside the go.mod of the module the templates name with
// the requirement init proposes applied, as the person applies it, and the
// go.sum lines of that requirement.
func renderedRepository(t *testing.T, capabilities []string) map[string][]byte {
	t.Helper()
	context := assetContext()
	context.Capabilities = capabilities
	context.DirectRequires = []assets.Requirement{{Path: "example.test/library", Version: "v1.2.3"}}
	rendered, err := assets.RenderAllContext(golaw.FS, context)
	if err != nil {
		t.Fatal(err)
	}
	module := "module " + context.ModulePath + "\n\ngo " + context.GoVersion + "\n\n"
	files := map[string][]byte{"go.sum": []byte(dslSums)}
	for i := range rendered {
		files[rendered[i].Path] = rendered[i].Data
	}
	files["go.mod"] = append([]byte(module), files["go.mod"]...)
	if _, ok := files["test/deadcode/reachability_test.go"]; !ok {
		t.Fatal("the deadcode asset is not rendered")
	}
	return files
}
