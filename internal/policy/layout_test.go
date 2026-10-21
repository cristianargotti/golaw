package policy_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// TestCompileCarriesTheLayout pins the layout the effective policy holds:
// what golaw.yml names, and the core default for each part it leaves out,
// so a check finds the decision records where the repository keeps them.
func TestCompileCarriesTheLayout(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, layout string
		want         model.Layout
	}{
		{"absent", "", model.Layout{Main: "root", Decisions: "docs/adr"}},
		{"both", "layout: {main: cmd, decisions: docs/decisions}\n", model.Layout{Main: "cmd", Decisions: "docs/decisions"}},
		{"decisions only", "layout: {decisions: records}\n", model.Layout{Main: "root", Decisions: "records"}},
		{"main only", "layout: {main: cmd}\n", model.Layout{Main: "cmd", Decisions: "docs/adr"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p, err := policy.Compile(decode(t, minimalPolicy+tc.layout), now(), schemas())
			if err != nil {
				t.Fatal(err)
			}
			if p.Layout != tc.want {
				t.Fatalf("layout = %+v, want %+v", p.Layout, tc.want)
			}
		})
	}
}
