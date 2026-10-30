package conformance_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/surfacefix"
)

func TestTheRedTwinReadsItsTestsAsMainCode(t *testing.T) {
	t.Parallel()
	scope, err := surface.ScopeOf(t.Context(), twin(t, surfacefix.Red), nil)
	if err != nil {
		t.Fatal(err)
	}
	if scope.Source != model.FromProperties || scope.CoverageReport != "profile.cov" || len(scope.Unmeasured) != 0 {
		t.Fatalf("scope = %+v, want the twin's properties", scope)
	}
	if want := []string{"**/*_test.go", "integration/**", "**/testdata/**"}; !slices.Equal(scope.Exclusions, want) {
		t.Errorf("Exclusions = %v, want %v", scope.Exclusions, want)
	}
	roles := map[string]surface.Role{
		"cmd/hub/main.go":             surface.RoleMain,
		"cmd/hub/main_test.go":        surface.RoleOutside,
		"integration/flow.go":         surface.RoleOutside,
		"test/view/host.test.cjs":     surface.RoleMain,
		"test/deploy/test_install.py": surface.RoleMain,
		"views/panel.html":            surface.RoleMain,
	}
	for path, want := range roles {
		if got := scope.Role(path); got != want {
			t.Errorf("Role(%q) = %d, want %d", path, got, want)
		}
	}
}
