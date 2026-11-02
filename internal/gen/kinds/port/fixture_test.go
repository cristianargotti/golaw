package port_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/goldentest"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/model"
)

const (
	module    = "example.com/slice"
	area      = "internal/application/growth/"
	portsPath = area + "ports.go"
	ledgerDir = area + "ledgertest/"
)

func ledger() genkind.Port {
	ctx := genkind.Param{Name: "ctx", Type: "context.Context"}
	account := genkind.Param{Name: "account", Type: "AccountID"}
	return genkind.Port{
		Name: "Ledger", Package: "growth", Path: portsPath, Dir: strings.TrimSuffix(area, "/"), Line: 10,
		Imports: []genkind.Import{{Path: "context"}, {Path: "time"}},
		Methods: []genkind.Method{
			{Name: "ReadPlan", Line: 12, Params: []genkind.Param{ctx, account}, Results: []genkind.Param{{Type: "Plan"}, {Type: "error"}}},
			{Name: "ArchivePlan", Line: 14, Params: []genkind.Param{ctx, account}, Results: []genkind.Param{{Type: "error"}}},
		},
	}
}

func clock() genkind.Port {
	p := ledger()
	p.Name, p.Line = "Clock", 18
	p.Methods = []genkind.Method{{Name: "Now", Line: 20, Results: []genkind.Param{{Type: "time.Time"}}}}
	return p
}

func input(t *testing.T, files map[string][]byte, entries []manifest.Entry, ports ...genkind.Port) genkind.Input {
	t.Helper()
	answers := make(map[string]genkind.Port, len(ports))
	for i := range ports {
		answers[ports[i].Path+"#"+ports[i].Name] = ports[i]
	}
	return genkind.Input{
		Module: module, Version: "0.3.0", Policy: model.DefaultGenerate(), Snapshot: gentest.Snapshot(t, files),
		Manifest:  manifest.Manifest{Schema: manifest.Schema, Support: manifest.Support{Path: "internal/gensupport", Template: "support@1"}, Entries: entries},
		Contracts: gentest.FakeContracts{PortAnswers: answers},
	}
}

func request(typ, usecase string) genkind.Request {
	return genkind.Request{Name: genkind.Name{Area: "growth", Type: typ}, Usecase: usecase}
}

func ledgerEntry() manifest.Entry {
	return manifest.Entry{
		Kind: genkind.KindPort, Template: "port@1", Name: "growth.Ledger",
		Contracts:   []manifest.Contract{{Path: portsPath, Symbol: "Ledger"}},
		Regenerable: []string{ledgerDir + "zz_generated_fake.go", ledgerDir + "zz_generated_suite.go", ledgerDir + "zz_generated_suite_test.go"},
		Owned:       []string{ledgerDir + "cases.go"},
	}
}

func readPlanEntry(ports ...string) manifest.Entry {
	return manifest.Entry{
		Kind: genkind.KindUsecase, Template: "usecase@1", Name: "growth.ReadPlan", Ports: ports, In: "AccountID", Out: "Plan",
		Regenerable: []string{area + "zz_generated_read_plan.go", area + "zz_generated_usecases.go", area + "zz_generated_usecases_test.go"},
		Owned:       []string{area + "read_plan.go", area + "read_plan_test.go"},
	}
}

func withFile(path, content string) map[string][]byte {
	files := gentest.Contracts()
	files[path] = []byte(content)
	return files
}

const billingPorts = "internal/application/billing/ports.go"

// growthClockEntry is the manifest entry of the fixture's Clock port.
func growthClockEntry() manifest.Entry {
	return manifest.Entry{
		Kind: genkind.KindPort, Template: "port@1", Name: "growth.Clock",
		Contracts:   []manifest.Contract{{Path: portsPath, Symbol: "Clock"}},
		Regenerable: []string{area + "clocktest/zz_generated_fake.go", area + "clocktest/zz_generated_suite.go", area + "clocktest/zz_generated_suite_test.go"},
		Owned:       []string{area + "clocktest/cases.go"},
	}
}

// billingClock is a Clock port of another area with one method of the
// standard library's types, so its fake would render byte for byte as the
// fixture's when the method is Now.
func billingClock(method string) genkind.Port {
	return genkind.Port{
		Name: "Clock", Package: "billing", Path: billingPorts, Dir: "internal/application/billing", Line: 6,
		Imports: []genkind.Import{{Path: "time"}},
		Methods: []genkind.Method{{Name: method, Line: 7, Results: []genkind.Param{{Type: "time.Time"}}}},
	}
}

func paths(files []genkind.File) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

func content(t *testing.T, files []genkind.File, path string) string {
	t.Helper()
	for _, f := range files {
		if f.Path == path {
			return string(f.Content)
		}
	}
	t.Fatalf("no file %s among %v", path, paths(files))
	return ""
}

func assertHas(t *testing.T, text string, wants ...string) {
	t.Helper()
	normal := strings.Join(strings.Fields(text), " ")
	for _, want := range wants {
		if !strings.Contains(normal, strings.Join(strings.Fields(want), " ")) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
}

func assertLaws(t *testing.T, out genkind.Output) {
	t.Helper()
	files := map[string][]byte{}
	for _, f := range slices.Concat(out.Regenerable, out.Owned) {
		files[f.Path] = f.Content
	}
	for _, law := range goldentest.Laws() {
		for _, violation := range law.Assert(files) {
			t.Errorf("%s: %s", law.Name, violation)
		}
	}
}
