package goast_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

const storedText = "package postgres\n\nfunc save(err error) string {\n\tmessage := err.Error()\n\treturn message\n}\n"

// layoutPolicy is the proof policy with a generate block declared, edited
// by the row.
func layoutPolicy(edit func(*model.GeneratePolicy)) *model.EffectivePolicy {
	p := proof.Policy()
	p.Generate = model.DefaultGenerate()
	p.Generate.Declared = true
	edit(&p.Generate)
	return p
}

func TestErrorTextAsDataReadsTheDeclaredLayout(t *testing.T) {
	t.Parallel()
	keep := func(*model.GeneratePolicy) {}
	tests := map[string]struct {
		policy *model.EffectivePolicy
		path   string
		want   int
	}{
		"red: an out adapter under the declared layout":      {layoutPolicy(keep), "internal/adapters/out/postgres/plans.go", 1},
		"red: a use case under a custom application root":    {layoutPolicy(func(g *model.GeneratePolicy) { g.Layout.Usecase = "internal/app/{area}" }), "internal/app/growth/read_plan.go", 1},
		"red: the jobs directories stay judged":              {layoutPolicy(keep), "internal/infrastructure/jobs/sync.go", 1},
		"near: an outbound adapter once the layout says out": {layoutPolicy(keep), "internal/adapters/outbound/postgres/plans.go", 0},
		"near: an out adapter without a declared block":      {proof.Policy(), "internal/adapters/out/postgres/plans.go", 0},
		"near: an inbound handler is no outbound layer":      {layoutPolicy(keep), "internal/adapters/in/api/handler.go", 0},
		"red: an outbound adapter without a declared block":  {proof.Policy(), "internal/adapters/outbound/postgres/plans.go", 1},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := goast.ErrorTextAsData()
			tree := proof.NewTree(map[string][]byte{tc.path: []byte(storedText)})
			res, err := c.Run(t.Context(), model.Input{
				Mode: model.ModeScan, Policy: tc.policy, Snapshot: tree, Targets: tree.Files(), Options: c.Spec().Options.Defaults(),
			})
			if err != nil {
				t.Fatal(err)
			}
			if res.Status.Kind != model.StatusMeasured || len(res.Findings) != tc.want {
				t.Errorf("status %s, findings %+v, want %d", res.Status.Kind, res.Findings, tc.want)
			}
		})
	}
}
