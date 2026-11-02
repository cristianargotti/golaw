package gen_test

import (
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
)

func TestCheckReportsEveryDriftCauseAndTheOpenHoles(t *testing.T) {
	t.Parallel()
	m := enrolled(plansEntry())
	tests := map[string]struct {
		edit    func(files map[string][]byte)
		appends []genkind.Append
		want    []gen.Drift
	}{
		"clean":                 {func(map[string][]byte) {}, nil, nil},
		"a hand edit":           {func(f map[string][]byte) { f[plansGen] = append(f[plansGen], "// edit\n"...) }, nil, []gen.Drift{{Path: plansGen, Cause: gen.DriftDiffers, Fix: gen.FixGen}}},
		"a missing file":        {func(f map[string][]byte) { delete(f, plansGen) }, nil, []gen.Drift{{Path: plansGen, Cause: gen.DriftMissing, Fix: gen.FixGen}}},
		"a missing stub":        {func(map[string][]byte) {}, []genkind.Append{archiveAppend()}, []gen.Drift{{Path: plansPath, Cause: gen.DriftStub, Fix: gen.FixGen}}},
		"a gone contract":       {func(f map[string][]byte) { delete(f, portsPath) }, nil, []gen.Drift{{Path: portsPath, Cause: gen.DriftContractGone, Fix: "git restore " + portsPath}}},
		"a stale manifest file": {func(f map[string][]byte) { delete(f, manifest.Path) }, nil, []gen.Drift{{Path: manifest.Path, Cause: gen.DriftMissing, Fix: gen.FixGen}}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			files := inSync(t, m)
			tc.edit(files)
			in := engineInput(t, files, m, supportKind(), regenerating(tc.appends...))
			in.Changed = []string{"README.md"}
			res, err := gen.Check(t.Context(), in)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(res.Drift, tc.want) || res.Writes != nil {
				t.Errorf("drift = %+v, writes %+v, want %+v and no write", res.Drift, res.Writes, tc.want)
			}
			if name != "a gone contract" && (len(res.Open) != 1 || res.Open[0].ID != "plans.read-plan") {
				t.Errorf("open = %+v, want the open hole of the owned file", res.Open)
			}
		})
	}
}
