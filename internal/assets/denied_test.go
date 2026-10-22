package assets_test

import (
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/assets"
)

// deniedInBaseline answers the modules the table of a rendered baseline
// record marks denied.
func deniedInBaseline(t *testing.T, data []byte) []string {
	t.Helper()
	var denied []string
	for line := range strings.SplitSeq(string(data), "\n") {
		cells := strings.Split(line, "|")
		if len(cells) == 5 && strings.TrimSpace(cells[3]) == "denied" {
			denied = append(denied, strings.TrimSpace(cells[1]))
		}
	}
	return denied
}

// deniedInDepguard answers the third-party modules the lax main rule of a
// rendered lint configuration denies; a standard library entry has no dot
// in its first element.
func deniedInDepguard(t *testing.T, data []byte) []string {
	t.Helper()
	var config map[string]any
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	rules := yamlMap(t, yamlMap(t, yamlMap(t, yamlMap(t, config, "linters"), "settings"), "depguard"), "rules")
	var modules []string
	for _, deny := range anyList(t, yamlMap(t, rules, "main")["deny"]) {
		fields, ok := deny.(map[string]any)
		if !ok {
			t.Fatal("deny entry is not a mapping")
		}
		if pkg, ok := fields["pkg"].(string); ok && strings.Contains(strings.SplitN(pkg, "/", 2)[0], ".") {
			modules = append(modules, pkg)
		}
	}
	return modules
}

func TestDeniedModulesAgreeBetweenTheBaselineAndDepguard(t *testing.T) {
	t.Parallel()
	context := assetContext()
	context.DirectRequires = []assets.Requirement{
		{Path: "github.com/gin-gonic/gin", Version: "v1.12.0"},
		{Path: "github.com/pkg/errors", Version: "v0.9.1"},
		{Path: "github.com/sirupsen/logrus", Version: "v1.10.2"},
		{Path: "github.com/swaggo/swag", Version: "v1.16.6"},
	}
	baseline := deniedInBaseline(t, renderedAsset(t, "docs/adr/0001-inherited-baseline.md", context).Data)
	if !slices.Equal(baseline, []string{"github.com/pkg/errors", "github.com/sirupsen/logrus", "github.com/swaggo/swag"}) {
		t.Fatalf("baseline denies %v", baseline)
	}
	modules := deniedInDepguard(t, renderedAsset(t, ".golangci.yml", context).Data)
	for _, path := range baseline {
		if !slices.ContainsFunc(modules, func(pkg string) bool { return path == pkg || strings.HasPrefix(path, pkg+"/") }) {
			t.Errorf("the baseline denies %s and depguard lets it in: %v", path, modules)
		}
	}
	for _, pkg := range modules {
		if !slices.ContainsFunc(baseline, func(path string) bool { return path == pkg || strings.HasPrefix(path, pkg+"/") }) {
			t.Errorf("depguard denies %s and the baseline records it inherited", pkg)
		}
	}
}
