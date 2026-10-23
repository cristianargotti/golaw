package assets_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestBuildAssetCarriesTheCapabilityTargets(t *testing.T) {
	t.Parallel()
	for _, caps := range [][]string{
		nil,
		{"image"},
		{"go"},
		{"api", "go"},
		{"db", "go"},
		{"go", "image"},
		{"go", "hexagonal"},
		model.Capabilities(),
	} {
		t.Run(strings.Join(caps, ","), func(t *testing.T) {
			t.Parallel()
			context := assetContext()
			context.Capabilities = caps
			body := string(renderedAsset(t, "golaw.mk", context).Data)
			ci := requireBuildCapabilityTargets(t, body, caps)
			requireBuildToolchains(t, body)
			requireBuildDemandContract(t, body, ci)
		})
	}
}

func requireBuildCapabilityTargets(t *testing.T, body string, caps []string) []string {
	t.Helper()
	var ci []string
	for line := range strings.SplitSeq(body, "\n") {
		if terms, found := strings.CutPrefix(line, "ci:"); found {
			ci = strings.Fields(terms)
		}
	}
	want := policyfiles.RequiredTargets(caps)
	if !slices.Equal(ci, want) {
		t.Fatalf("ci = %v, want %v", ci, want)
	}
	for _, target := range want {
		if !strings.Contains(body, "\n"+target+":") {
			t.Errorf("target %s has no definition", target)
		}
	}
	return ci
}

func requireBuildToolchains(t *testing.T, body string) {
	t.Helper()
	target := ""
	for line := range strings.SplitSeq(body, "\n") {
		if name, _, found := strings.Cut(line, ":"); found && !strings.HasPrefix(line, "\t") &&
			!strings.Contains(line, "export") && !strings.Contains(name, " ") {
			target = name
		}
		if strings.HasPrefix(line, "\t") && strings.Contains(line, "go run ") &&
			!strings.Contains(body, "\n"+target+": export GOTOOLCHAIN := auto\n") {
			t.Errorf("target %s runs a pinned tool under the local toolchain: %q", target, line)
		}
		if strings.HasPrefix(line, "\tGOTOOLCHAIN=") {
			t.Errorf("a recipe sets the toolchain through the shell, which cmd.exe cannot run: %q", line)
		}
	}
}

func requireBuildDemandContract(t *testing.T, body string, ci []string) {
	t.Helper()
	if !strings.Contains(body, "\nmutation-changed:") || slices.Contains(ci, "mutation-changed") {
		t.Errorf("mutation-changed must exist and stay on demand: ci = %v", ci)
	}
	for _, fragment := range []string{
		"export GOTOOLCHAIN=local\n",
		"GOLAW_ENROLLED ?=\n",
		"golaw scan $(if $(GOLAW_ENROLLED),--enrolled $(GOLAW_ENROLLED)) .",
		"tools/depsage@v0.2.0 -days 7",
		"tools/vulngate@v0.2.0 -scan -waivers golaw.yml",
		"gremlins/cmd/gremlins@v0.6.0 unleash --diff $(MUTATION_BASE)",
		"MUTATION_BASE ?= origin/main\n",
	} {
		if !strings.Contains(body, fragment) {
			t.Errorf("missing build contract: %s", fragment)
		}
	}
}

// TestBuildAssetLicensesJudgeTheDependenciesAlone holds the licenses target
// to the modules the repository depends on: the repository's own packages
// carry the license it chose, a private one none, and go-licenses reports a
// package without a license file as an error.
func TestBuildAssetLicensesJudgeTheDependenciesAlone(t *testing.T) {
	t.Parallel()
	body := string(renderedAsset(t, "golaw.mk", assetContext()).Data)
	want := "\tgo run github.com/google/go-licenses/v2@v2.0.1 check --disallowed_types=forbidden,restricted --ignore example.test/service ./...\n"
	if !strings.Contains(body, want) {
		t.Fatalf("the licenses recipe does not ignore the module's own packages:\n%s", body)
	}
}

// TestBuildAssetLicensesStateTheirPrefixLimit pins the limit of the ignore
// beside the recipe: go-licenses compares an ignored path as a plain prefix,
// so a sibling module whose path extends the module's as a string is skipped
// too, and docs/rules.md must say so while the recipe ignores the bare path.
func TestBuildAssetLicensesStateTheirPrefixLimit(t *testing.T) {
	t.Parallel()
	body := string(renderedAsset(t, "golaw.mk", assetContext()).Data)
	if !strings.Contains(body, "--ignore example.test/service ./...") {
		t.Fatalf("the licenses recipe changed its ignore; update the limit docs/rules.md states:\n%s", body)
	}
	rules, err := os.ReadFile(filepath.Join("..", "..", "docs", "rules.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"go-licenses compares an ignored path as a plain string prefix with no path boundary",
		"such as github.com/acme/svc-client beside github.com/acme/svc, is skipped too",
		"review the license of such a sibling module by hand",
	} {
		if !strings.Contains(string(rules), fragment) {
			t.Errorf("docs/rules.md lacks the licenses prefix limit %q", fragment)
		}
	}
}
