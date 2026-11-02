package support_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/goldentest"
	"github.com/cristianargotti/golaw/internal/gen/kinds/support"
	"github.com/cristianargotti/golaw/internal/gentest"
)

func TestGoldenSliceSupport(t *testing.T) {
	t.Parallel()
	report := goldentest.Run(t, goldentest.Slice{
		Name: "support", Module: "example.com/slice", Base: gentest.Contracts(),
		Kinds:    []genkind.Kind{support.Kind()},
		Commands: []goldentest.Command{{Kind: genkind.KindSupport}},
	}, goldentest.StepBuild, goldentest.StepLint, goldentest.StepScan, goldentest.StepSurface, goldentest.StepCoverage,
		goldentest.StepDup, goldentest.StepFormat, goldentest.StepLaws, goldentest.StepFloor)
	for _, path := range []string{
		"internal/gensupport/zz_generated_errors.go", "internal/gensupport/zz_generated_hole.go", "internal/gensupport/zz_generated_recorder.go",
	} {
		if len(report.Files[path]) == 0 {
			t.Errorf("the slice lacks %s", path)
		}
	}
	if _, skipped := report.Skipped[goldentest.StepCoverage]; !skipped && (report.Coverage < 100 || !report.OpenCovered) {
		t.Errorf("coverage %.1f, open covered %v: the support package is covered line for line by its own tests", report.Coverage, report.OpenCovered)
	}
	if report.Duplication >= 1.5 {
		t.Errorf("duplication %.2f percent, the golden threshold is 1.5", report.Duplication)
	}
}

func TestSupportShapesKeepTheTemplateLaws(t *testing.T) {
	t.Parallel()
	goldentest.Prove(t, "testdata/proof.yml", goldentest.Laws())
}
