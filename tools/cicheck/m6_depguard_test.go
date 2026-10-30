package cicheck_test

import (
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestDepguardDrawsTheSurfaceFamilyDirection holds the native checks of the
// non-Go surface to the model, the surface model and YAML, the imports the
// lexers and the CloudFormation reader need, while every other family keeps
// the model alone.
func TestDepguardDrawsTheSurfaceFamilyDirection(t *testing.T) {
	t.Parallel()
	var config lintConfig
	if err := yaml.Unmarshal(kitBytes(t, ".golangci.yml"), &config); err != nil {
		t.Fatal(err)
	}
	rules := config.Linters.Settings.Depguard.Rules
	family, ok := rules["surface-family"]
	wantFiles := []string{"!$test", "**/internal/checks/surface/**"}
	wantAllow := []string{"$gostd", kitModule + "model", kitModule + "surface", "gopkg.in/yaml.v3"}
	if !ok || family.ListMode != "strict" || !slices.Equal(family.Files, wantFiles) || !slices.Equal(family.Allow, wantAllow) {
		t.Errorf("rule surface-family = %+v, want strict over %v allowing %v", family, wantFiles, wantAllow)
	}
	families := rules["families"]
	if !slices.Contains(families.Files, "!**/internal/checks/surface/**") || !slices.Equal(families.Allow, []string{"$gostd", kitModule + "model"}) {
		t.Errorf("rule families = %+v, want the surface family left to its own rule", families)
	}
}
