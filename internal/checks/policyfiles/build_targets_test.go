package policyfiles_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestRequiredTargetsFollowCapabilitiesAndLadderOrder(t *testing.T) {
	t.Parallel()
	const core = "mod deps-age fmt vet generate-check test coverage-check coverage-new build lint deadcode"
	const tail = "vuln licenses secrets scan lint-surface"
	for _, tc := range []struct {
		name string
		caps []string
		want string
	}{
		{"near no capability", nil, "scan lint-surface"},
		{"near go", []string{"go"}, core + " " + tail},
		{"near cli", []string{"cli", "go"}, core + " " + tail},
		{"near hexagonal", []string{"go", "hexagonal"}, core + " " + tail},
		{"red db omission", []string{"db", "go"}, core + " migrate-lint " + tail},
		{"red api omission", []string{"api", "go"}, core + " openapi-breaking openapi-examples-check vuln licenses docs-check secrets scan lint-surface"},
		{"near image alone", []string{"image"}, "scan lint-surface dockerfile-lint image-pins-check image image-scan"},
		{"red all capabilities", model.Capabilities(), core + " migrate-lint openapi-breaking openapi-examples-check vuln licenses docs-check secrets scan lint-surface dockerfile-lint image-pins-check image image-scan"},
		{"near repeated input", []string{"go", "go"}, core + " " + tail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			before := slices.Clone(tc.caps)
			got := policyfiles.RequiredTargets(tc.caps)
			if !slices.Equal(got, strings.Fields(tc.want)) {
				t.Fatalf("targets = %v, want %s", got, tc.want)
			}
			if !slices.Equal(before, tc.caps) {
				t.Fatal("capabilities were mutated")
			}
			for _, absent := range []string{"hooks-test", "mutation-changed", "fuzz", "bench", "test-integration"} {
				if slices.Contains(got, absent) {
					t.Fatalf("core CI unexpectedly requires %s", absent)
				}
			}
		})
	}
}

func TestRequiredTargetsReturnsIndependentData(t *testing.T) {
	t.Parallel()
	first := policyfiles.RequiredTargets([]string{"go"})
	first[0] = "changed"
	second := policyfiles.RequiredTargets([]string{"go"})
	if second[0] != "mod" {
		t.Fatalf("shared mutable targets: %v", second)
	}
}
