package adapterhttp_test

import (
	"bytes"
	"os"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/goldentest"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adapterhttp"
	"github.com/cristianargotti/golaw/internal/gen/kinds/port"
	"github.com/cristianargotti/golaw/internal/gen/kinds/support"
	"github.com/cristianargotti/golaw/internal/gen/kinds/usecase"
	"github.com/cristianargotti/golaw/internal/gen/kinds/wiring"
	"github.com/cristianargotti/golaw/internal/gentest"
)

func fills(t *testing.T, paths ...string) []goldentest.Fill {
	t.Helper()
	var out []goldentest.Fill
	at := map[string]int{}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var list []goldentest.Fill
		if err := yaml.Unmarshal(data, &list); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		// The use case fills repeat the port fills byte for byte, so one
		// fill per id is kept and a later file wins, as the handler loader.
		for _, f := range list {
			if i, ok := at[f.ID]; ok {
				out[i] = f
				continue
			}
			at[f.ID] = len(out)
			out = append(out, f)
		}
	}
	return out
}

func TestGoldenSliceAdapterHTTP(t *testing.T) {
	t.Parallel()
	slice := goldentest.Slice{
		Name: genkind.KindAdapterHTTP, Module: sliceModule, Base: gentest.Contracts(),
		Kinds: []genkind.Kind{support.Kind(), port.Kind(), usecase.Kind(), adapterhttp.Kind(), wiring.Kind()},
		Commands: []goldentest.Command{
			{Kind: genkind.KindUsecase, Request: genkind.Request{Name: genkind.Name{Area: "growth", Type: "ReadPlan"}, Ports: []string{"Ledger", "Clock"}, In: "AccountID", Out: "Plan"}},
			{Kind: genkind.KindUsecase, Request: genkind.Request{Name: genkind.Name{Area: "growth", Type: "ArchivePlan"}, Ports: []string{"Ledger"}, In: "AccountID", Out: "Plan"}},
			{Kind: genkind.KindAdapterHTTP, Request: request("Client")},
			{Kind: genkind.KindAdapterHTTP, Request: request("Mirror")},
		},
		Fills: fills(t, "testdata/fills.yml", "../port/testdata/fills.yml", "../usecase/testdata/fills.yml", "../wiring/testdata/fills.yml"),
	}
	report := goldentest.Run(t, slice,
		goldentest.StepBuild, goldentest.StepLint, goldentest.StepScan, goldentest.StepSurface, goldentest.StepCoverage,
		goldentest.StepDup, goldentest.StepCheck, goldentest.StepFormat, goldentest.StepLaws, goldentest.StepFloor)
	if _, skipped := report.Skipped[goldentest.StepCoverage]; !skipped && (report.Coverage < 80 || !report.OpenCovered) {
		t.Errorf("coverage on new code = %.1f, every regenerable line covered with the holes open = %v", report.Coverage, report.OpenCovered)
	}
	if _, skipped := report.Skipped[goldentest.StepDup]; !skipped && report.Duplication >= 1.5 {
		t.Errorf("duplication on new lines = %.2f percent, the golden threshold is 1.5", report.Duplication)
	}
	for p, content := range report.Files {
		if bytes.Contains(content, []byte("http:"+"//")) {
			t.Errorf("%s holds a clear text URL", p)
		}
	}
}

func TestAdapterHTTPProof(t *testing.T) {
	t.Parallel()
	goldentest.Prove(t, "testdata/proof.yml", goldentest.Laws())
}
