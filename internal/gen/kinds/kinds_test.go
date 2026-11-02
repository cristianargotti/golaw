package kinds_test

import (
	"io/fs"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/goldentest"
	"github.com/cristianargotti/golaw/internal/gen/kinds"
	"github.com/cristianargotti/golaw/internal/gentest"
)

func TestAllListsTheSevenKindsInNameOrder(t *testing.T) {
	t.Parallel()
	names := make([]string, 0, len(kinds.All()))
	for _, k := range kinds.All() {
		names = append(names, k.Name())
		if k.Template() != 1 {
			t.Errorf("%s template = %d, phase M7.1 ships @1", k.Name(), k.Template())
		}
	}
	want := []string{
		genkind.KindAdapterHTTP, genkind.KindAdapterSQL, genkind.KindHandler, genkind.KindPort,
		genkind.KindSupport, genkind.KindUsecase, genkind.KindWiring,
	}
	if !slices.Equal(names, want) {
		t.Errorf("All = %v, want %v", names, want)
	}
	if k, ok := kinds.Lookup(genkind.KindPort); !ok || k.Name() != genkind.KindPort {
		t.Errorf("Lookup(port) = %v, %v", k, ok)
	}
	if _, ok := kinds.Lookup("queue"); ok {
		t.Error("Lookup found a kind phase M7.1 does not ship")
	}
	if _, ok := kinds.Lookup(genkind.KindWiring); !ok {
		t.Error("the wiring kind is not registered")
	}
}

func TestEveryTemplateBelongsToARegisteredKindAndVersion(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	err := fs.WalkDir(golaw.FS, strings.TrimSuffix(assets.GenPrefix, "/"), func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		parts := strings.SplitN(strings.TrimPrefix(name, assets.GenPrefix), "/", 3)
		k, ok := kinds.Lookup(parts[0])
		if len(parts) != 3 || !ok || strconv.Itoa(k.Template()) != parts[1] || !strings.HasSuffix(name, ".tmpl") {
			t.Errorf("%s belongs to no registered kind and version", name)
		}
		seen[parts[0]] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range kinds.All() {
		if !seen[k.Name()] {
			t.Errorf("kind %s embeds no template", k.Name())
		}
	}
}

// fills reads the recorded fills each kind keeps beside its package, one
// fill per id: the use case fills repeat the port fills byte for byte.
func fills(t *testing.T) []goldentest.Fill {
	t.Helper()
	var out []goldentest.Fill
	at := map[string]int{}
	for _, dir := range []string{"adapterhttp", "adaptersql", "handler", "port", "usecase", "wiring"} {
		data, err := os.ReadFile(path.Join(dir, "testdata", "fills.yml"))
		if err != nil {
			t.Fatalf("kind %s keeps no recorded fills: %v", dir, err)
		}
		var kind []goldentest.Fill
		if err := yaml.Unmarshal(data, &kind); err != nil {
			t.Fatalf("%s/testdata/fills.yml: %v", dir, err)
		}
		for _, f := range kind {
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

// commands open exactly the holes the recorded fills close: the two use
// cases of the growth slice, whose needs create the Ledger and Clock ports,
// the two instances of each adapter kind over the Ledger port as their own
// slices write them, and the handler over the area.
func commands() []goldentest.Command {
	growth := func(name string) genkind.Name { return genkind.Name{Area: "growth", Type: name} }
	adapter := func(kind, area, name string) goldentest.Command {
		return goldentest.Command{Kind: kind, Request: genkind.Request{Name: genkind.Name{Area: area, Type: name}, Port: growth("Ledger")}}
	}
	return []goldentest.Command{
		{Kind: genkind.KindUsecase, Request: genkind.Request{Name: growth("ReadPlan"), Ports: []string{"Ledger", "Clock"}, In: "AccountID", Out: "Plan"}},
		{Kind: genkind.KindUsecase, Request: genkind.Request{Name: growth("ArchivePlan"), Ports: []string{"Ledger"}, In: "AccountID", Out: "Plan"}},
		adapter(genkind.KindAdapterSQL, "postgres", "Plans"),
		adapter(genkind.KindAdapterSQL, "postgres", "Replica"),
		adapter(genkind.KindAdapterHTTP, "planapi", "Client"),
		adapter(genkind.KindAdapterHTTP, "planapi", "Mirror"),
		{Kind: genkind.KindHandler, Request: genkind.Request{Name: genkind.Name{Area: "plansapi"}, API: "api/openapi.yaml", Usecases: []string{"growth"}}},
	}
}

// The slice of M7.13 row 1 inside the kit's tests: every kind of phase
// M7.1 in one module, the holes open and then filled.
func TestGoldenSliceEveryKind(t *testing.T) {
	t.Parallel()
	report := goldentest.Run(t, goldentest.Slice{
		Name: "every-kind", Module: "example.com/slice", Base: gentest.Contracts(),
		Kinds: kinds.All(), Commands: commands(), Fills: fills(t),
	}, goldentest.StepBuild, goldentest.StepLint, goldentest.StepScan, goldentest.StepSurface, goldentest.StepCoverage,
		goldentest.StepDup, goldentest.StepCheck, goldentest.StepFormat, goldentest.StepLaws, goldentest.StepFloor)
	wired := false
	for name := range report.Files {
		wired = wired || strings.HasSuffix(name, "/zz_generated_wiring.go") || name == "zz_generated_wiring.go"
	}
	if !wired || len(report.Files[".golaw/gen.yml"]) == 0 {
		t.Errorf("the slice lacks its wiring or its manifest: %d files", len(report.Files))
	}
	if _, skipped := report.Skipped[goldentest.StepCoverage]; !skipped && (report.Coverage < 80 || !report.OpenCovered) {
		t.Errorf("new code coverage %.1f with the fills, regenerable lines covered with the holes open %v", report.Coverage, report.OpenCovered)
	}
	if report.Duplication >= 1.5 {
		t.Errorf("duplication %.2f percent, the golden threshold is 1.5", report.Duplication)
	}
}
