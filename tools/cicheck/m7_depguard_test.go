package cicheck_test

import (
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestDepguardDrawsTheGenFamily holds the gen family to model, the header
// grammar and the manifest, so a guard never reaches the engine or the hook.
func TestDepguardDrawsTheGenFamily(t *testing.T) {
	t.Parallel()
	var config lintConfig
	if err := yaml.Unmarshal(kitBytes(t, ".golangci.yml"), &config); err != nil {
		t.Fatal(err)
	}
	rules := config.Linters.Settings.Depguard.Rules
	genguard, ok := rules["genguard"]
	if !ok || genguard.ListMode != "strict" || !slices.Equal(genguard.Files, []string{"!$test", "**/internal/checks/genguard/**"}) {
		t.Fatalf("genguard rule = %+v", genguard)
	}
	want := []string{"$gostd", kitModule + "model", kitModule + "genheader", kitModule + "gen/manifest", kitModule + "checks/shell"}
	if !slices.Equal(genguard.Allow, want) {
		t.Errorf("genguard allows %v, want %v", genguard.Allow, want)
	}
	if !slices.Contains(rules["families"].Files, "!**/internal/checks/genguard/**") {
		t.Errorf("the families rule still holds the gen family: %v", rules["families"].Files)
	}
}
