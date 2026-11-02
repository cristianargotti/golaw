package gen_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gentest"
)

var errRender = errors.New("render failed")

// ledgerPort is a port kind whose new instance writes nothing but its entry.
func ledgerPort() fakeKind {
	return fakeKind{name: genkind.KindPort, out: genkind.Output{
		Entry: manifest.Entry{Kind: genkind.KindPort, Template: "port@1", Name: "growth.Ledger"},
	}}
}

var ledger = genkind.Request{Name: genkind.Name{Area: "growth", Type: "Ledger"}}

func TestNewWiresTheBindingsOfTheEntriesItDidNotRender(t *testing.T) {
	t.Parallel()
	gone := manifest.Entry{
		Kind: genkind.KindAdapterSQL, Template: "adapter-sql@1", Name: "postgres.Audits",
		Contracts: []manifest.Contract{{Path: "internal/application/audit/ports.go", Symbol: "Audit"}},
	}
	m := enrolled(plansEntry(), gone)
	in := engineInput(t, inSync(t, m), m, supportKind(), regenerating(), ledgerPort(), wirer())
	res, err := gen.New(t.Context(), in, genkind.KindPort, ledger)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(writeOf(t, res, wiringPath).Content), "binds 1 values") {
		t.Errorf("the wiring lacks the binding of the untouched adapter: %+v", res.Writes)
	}
	broken := regenerating()
	broken.err = errRender
	in = engineInput(t, inSync(t, m), m, supportKind(), broken, ledgerPort(), wirer())
	if _, err := gen.New(t.Context(), in, genkind.KindPort, ledger); !errors.Is(err, errRender) || !strings.Contains(err.Error(), "postgres.Plans") {
		t.Errorf("a failed rendering of an untouched entry = %v", err)
	}
}

func TestNewNamesTheFirstFileOfAnExistingEntry(t *testing.T) {
	t.Parallel()
	regen := manifest.Entry{Kind: genkind.KindPort, Template: "port@1", Name: "growth.Ledger", Regenerable: []string{"internal/application/growth/zz_generated_ledger.go"}}
	bare := manifest.Entry{Kind: genkind.KindPort, Template: "port@1", Name: "growth.Ledger"}
	for want, e := range map[string]manifest.Entry{regen.Regenerable[0]: regen, manifest.Path: bare} {
		_, err := gen.New(t.Context(), engineInput(t, gentest.Contracts(), enrolled(e), supportKind(), ledgerPort()), genkind.KindPort, ledger)
		if !errors.Is(err, genkind.ErrExists) || !strings.HasSuffix(err.Error(), want) {
			t.Errorf("New over %+v = %v, want a refusal naming %s", e, err, want)
		}
	}
}

func TestNewRefusesATemplateTheKitDoesNotEmbed(t *testing.T) {
	t.Parallel()
	port := ledgerPort()
	port.out.Entry.Template = "port@2"
	_, err := gen.New(t.Context(), engineInput(t, gentest.Contracts(), empty(), supportKind(), port), genkind.KindPort, ledger)
	if !errors.Is(err, gen.ErrUnknownTemplate) || !strings.Contains(err.Error(), "growth.Ledger") {
		t.Errorf("New = %v", err)
	}
	support := supportKind()
	support.err = errRender
	if _, err := gen.New(t.Context(), engineInput(t, gentest.Contracts(), empty(), support), genkind.KindSupport, genkind.Request{}); !errors.Is(err, errRender) {
		t.Errorf("a failed support rendering = %v", err)
	}
}

// A manifest always names its support package, so a kit without the
// support kind refuses to write one instead of writing an invalid file.
func TestNewWithoutTheSupportKindWritesNoManifest(t *testing.T) {
	t.Parallel()
	_, err := gen.New(t.Context(), engineInput(t, gentest.Contracts(), empty(), ledgerPort()), genkind.KindPort, ledger)
	if !errors.Is(err, manifest.ErrManifest) || !strings.Contains(err.Error(), "support.path") {
		t.Errorf("New = %v, want the manifest refused for its missing support package", err)
	}
}

func TestGenerateDecidesOnAGoneOwnedFile(t *testing.T) {
	t.Parallel()
	// The entry keeps one owned file, so the run decides on the gone one;
	// deleting every owned file removes the entry instead (remove_test.go).
	entry := plansEntry()
	entry.Owned = append(entry.Owned, "internal/adapters/out/postgres/plans_test.go")
	m := enrolled(entry)
	files := inSync(t, m)
	files["internal/adapters/out/postgres/plans_test.go"] = []byte("package postgres\n")
	delete(files, plansPath)
	stubsOnly := regenerating(archiveAppend())
	stubsOnly.regen.Owned = nil
	res, err := gen.Generate(t.Context(), engineInput(t, files, m, supportKind(), stubsOnly))
	if err != nil || len(res.Decisions) != 1 || !strings.Contains(res.Decisions[0].Text, "plans.archive-plan were not appended") || res.Writes != nil {
		t.Fatalf("decisions = %+v, writes %+v, %v", res.Decisions, res.Writes, err)
	}
	res, err = gen.Generate(t.Context(), engineInput(t, files, m, supportKind(), regenerating(archiveAppend())))
	if err != nil || len(res.Writes) != 1 || res.Writes[0].Path != plansPath || res.Writes[0].Mode != gen.WriteOwned || len(res.Opened) != 2 {
		t.Errorf("writes = %+v, opened %+v, want the owned file created with every stub", res.Writes, res.Opened)
	}
	check, err := gen.Check(t.Context(), engineInput(t, files, m, supportKind(), regenerating()))
	if err != nil || len(check.Drift) != 1 || check.Drift[0] != (gen.Drift{Path: plansPath, Cause: gen.DriftMissing, Fix: gen.FixGen}) {
		t.Errorf("check = %+v, %v", check.Drift, err)
	}
}

func TestGenerateRecordsAnEntryTheKindRendersDifferently(t *testing.T) {
	t.Parallel()
	m := enrolled(plansEntry())
	kind := regenerating()
	kind.regen.Entry.Regenerable = []string{plansGen, "internal/adapters/out/postgres/zz_generated_plans_test.go"}
	res, err := gen.Generate(t.Context(), engineInput(t, inSync(t, m), m, supportKind(), kind))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := res.Manifest.Lookup(genkind.KindAdapterSQL, "postgres.Plans")
	if !ok || len(got.Regenerable) != 2 || writeOf(t, res, manifest.Path).Mode != gen.WriteManifest {
		t.Errorf("manifest = %+v, writes %+v", res.Manifest, res.Writes)
	}
}

// Red rows: what stops a run instead of being written over.
func TestGenerateStopsAtWhatItCannotRender(t *testing.T) {
	t.Parallel()
	m := enrolled(plansEntry())
	broken := regenerating()
	broken.err = errRender
	unparsable := inSync(t, m)
	unparsable[plansPath] = []byte("package postgres\n\nfunc {\n")
	newerSupport := enrolled(plansEntry())
	newerSupport.Support.Template = "support@2"
	failingSupport := supportKind()
	failingSupport.err = errRender
	tests := map[string]struct {
		in   gen.Input
		want string
	}{
		"a failed rendering":                {engineInput(t, inSync(t, m), m, supportKind(), broken), "postgres.Plans"},
		"an owned file that does not parse": {engineInput(t, unparsable, m, supportKind(), regenerating(archiveAppend())), plansPath},
		"a newer support template":          {engineInput(t, inSync(t, newerSupport), newerSupport, supportKind(), regenerating()), "support@2"},
		"a failed support":                  {engineInput(t, inSync(t, m), m, failingSupport, regenerating()), "support"},
		"the holes of a broken file":        {engineInput(t, unparsable, m, supportKind(), regenerating()), plansPath},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := gen.Generate(t.Context(), tc.in); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Generate = %v, want an error naming %s", err, tc.want)
			}
			if _, err := gen.Check(t.Context(), tc.in); err == nil {
				t.Errorf("Check answered no error")
			}
		})
	}
}

func TestGenerateWithoutAnyEntryWritesNoManifest(t *testing.T) {
	t.Parallel()
	res, err := gen.Generate(t.Context(), engineInput(t, gentest.Contracts(), empty(), supportKind(), wirer()))
	if err != nil || res.Writes != nil {
		t.Errorf("Generate = %+v, %v", res.Writes, err)
	}
}
