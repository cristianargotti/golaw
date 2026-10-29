package surface_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/surface"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestChecksListsTheFamilyInReportOrder(t *testing.T) {
	t.Parallel()
	want := []string{"surface.code-eval", "surface.hardcoded-credential", "surface.file-length", "surface.python-function-length"}
	family := surface.Checks()
	got := make([]string, 0, len(family))
	for _, c := range family {
		s := c.Spec()
		if err := s.Validate(); err != nil {
			t.Errorf("%s: %v", s.ID, err)
		}
		if s.Rule != "rules/4.16@1" || s.Owner != model.OwnerCore || s.Proof != s.ID || len(s.Needs) != 0 {
			t.Errorf("spec = %+v, want rules/4.16@1, core, its own proof key and no capability", s)
		}
		got = append(got, s.ID)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ids = %q, want %q", got, want)
	}
}
