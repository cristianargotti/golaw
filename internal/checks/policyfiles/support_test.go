// The helpers every black-box test of the policy family shares: a tree, the
// policy it is measured under, and one check dispatched over it through a
// registry of that check alone, so applicability by scope, owner and
// capability holds as it does in production.
package policyfiles_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// fixture is one tree and the policy a check measures it under.
type fixture struct {
	files        map[string]string // slash separated path to content
	capabilities []string          // the enabled capabilities, none when nil
	profile      string            // the enrolled profile, none when empty
	mode         model.Mode        // model.ModeScan when zero
	changes      []model.Change    // the staged changes a staged check reads
	options      model.Options     // the policy values of the check's options
	layout       *model.Layout     // the core default when nil
}

// measure dispatches the check over the fixture and answers its result.
func measure(t *testing.T, c model.Check, f fixture) model.Result {
	t.Helper()
	b := engine.New()
	if err := b.Register(c); err != nil {
		t.Fatal(err)
	}
	reg, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string][]byte, len(f.files))
	for path, content := range f.files {
		files[path] = []byte(content)
	}
	tree := proof.NewTree(files)
	run, err := engine.Dispatch(t.Context(), reg, model.Input{
		Mode: modeOf(f), Policy: policyOf(c, f), Snapshot: tree, Targets: tree.Files(), Changes: f.changes,
	})
	if err != nil {
		t.Fatal(err)
	}
	return run.Outcomes[0].Result
}

// policyOf is the core policy with the fixture's capabilities, profile,
// options and layout.
func policyOf(c model.Check, f fixture) *model.EffectivePolicy {
	p := model.CoreDefaults()
	p.Digest = "fixture"
	p.Capabilities = slices.Sorted(slices.Values(f.capabilities))
	p.Profile = f.profile
	p.Options = map[string]model.Options{c.Spec().ID: f.options}
	if f.layout != nil {
		p.Layout = *f.layout
	}
	return &p
}

func modeOf(f fixture) model.Mode {
	if f.mode == 0 {
		return model.ModeScan
	}
	return f.mode
}

// codes answers the finding codes of a result, sorted.
func codes(res model.Result) []string {
	out := make([]string, 0, len(res.Findings))
	for _, f := range res.Findings {
		out = append(out, f.Code)
	}
	slices.Sort(out)
	return out
}
