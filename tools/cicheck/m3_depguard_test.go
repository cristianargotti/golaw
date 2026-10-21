package cicheck_test

import (
	"bytes"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestDepguardDrawsThePolicyFilesDirection holds the policy family to the
// imports contract D16 names: the model, yaml.v3 for the YAML files, x/mod
// for go.mod and the TOML decoder for the gitleaks and squawk files, while
// every other family keeps the model alone.
func TestDepguardDrawsThePolicyFilesDirection(t *testing.T) {
	t.Parallel()
	var config lintConfig
	if err := yaml.Unmarshal(kitBytes(t, ".golangci.yml"), &config); err != nil {
		t.Fatal(err)
	}
	rules := config.Linters.Settings.Depguard.Rules
	policy, ok := rules["policyfiles"]
	wantFiles := []string{"!$test", "**/internal/checks/policyfiles/**"}
	wantAllow := []string{"$gostd", kitModule + "model", "gopkg.in/yaml.v3", "golang.org/x/mod", "github.com/BurntSushi/toml"}
	if !ok || policy.ListMode != "strict" || !slices.Equal(policy.Files, wantFiles) || !slices.Equal(policy.Allow, wantAllow) {
		t.Errorf("rule policyfiles = %+v, want strict over %v allowing %v", policy, wantFiles, wantAllow)
	}
	families := rules["families"]
	if !slices.Contains(families.Files, "!**/internal/checks/policyfiles/**") || !slices.Equal(families.Allow, []string{"$gostd", kitModule + "model"}) {
		t.Errorf("rule families = %+v, want the policy family left to its own rule", families)
	}
}

// TestLintConfigKeepsTheFileLimit holds .golangci.yml to the 300 line law
// of section 3, which every rule a later task adds must keep.
func TestLintConfigKeepsTheFileLimit(t *testing.T) {
	t.Parallel()
	if lines := bytes.Count(kitBytes(t, ".golangci.yml"), []byte("\n")); lines > 300 {
		t.Errorf(".golangci.yml has %d lines, want at most 300", lines)
	}
}
