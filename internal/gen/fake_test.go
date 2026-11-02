package gen_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/genheader"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/model"
)

const (
	portsPath  = "internal/application/growth/ports.go"
	plansPath  = "internal/adapters/out/postgres/plans.go"
	plansGen   = "internal/adapters/out/postgres/zz_generated_plans.go"
	holePath   = "internal/gensupport/zz_generated_hole.go"
	wiringPath = "cmd/api/zz_generated_wiring.go"
	gensupport = "example.com/slice/internal/gensupport"
)

// ownedPlans is the owned file of the fake SQL adapter: one open hole.
const ownedPlans = "package postgres\n\nimport \"example.com/slice/internal/gensupport\"\n\n// ReadPlan reads a plan.\nfunc ReadPlan() error {\n" +
	"\t// golaw:hole plans.read-plan call p.queries.ReadPlan\n\treturn gensupport.Hole(\"plans.read-plan\")\n}\n"

const (
	readStub    = "// ReadPlan reads a plan.\nfunc ReadPlan() error {\n\treturn nil\n}\n"
	archiveStub = "// ArchivePlan archives a plan.\nfunc ArchivePlan() error {\n\t// golaw:hole plans.archive-plan call p.queries.ArchivePlan\n" +
		"\treturn gensupport.Hole(\"plans.archive-plan\")\n}\n"
)

func header(kind, version string) string {
	return genheader.Header{Version: version, Kind: kind, Template: 1}.Line()
}

func plansEntry() manifest.Entry {
	return manifest.Entry{
		Kind: genkind.KindAdapterSQL, Template: "adapter-sql@1", Name: "postgres.Plans",
		Contracts:   []manifest.Contract{{Path: portsPath, Symbol: "Ledger"}},
		Regenerable: []string{plansGen}, Owned: []string{plansPath},
	}
}

func plansGenerated(version string) []byte {
	return []byte(header(genkind.KindAdapterSQL, version) + "\n\npackage postgres\n")
}

func plansOutput() genkind.Output {
	return genkind.Output{
		Entry:       plansEntry(),
		Regenerable: []genkind.File{{Path: plansGen, Content: plansGenerated("0.3.0")}},
		Owned:       []genkind.File{{Path: plansPath, Content: []byte(ownedPlans)}},
		Bindings:    []genkind.Binding{{Kind: genkind.KindAdapterSQL, Area: "growth", Port: "Ledger", Call: "postgres.NewPlans(infra.DB)"}},
	}
}

// archiveAppend is the stub of a method the port gained, beside one the
// owned file already declares.
func archiveAppend() genkind.Append {
	return genkind.Append{
		Path: plansPath, Decls: [][]byte{[]byte(readStub), []byte(archiveStub)},
		Imports: []genkind.Import{{Path: gensupport}}, Holes: []string{"plans.read-plan", "plans.archive-plan"},
	}
}

// fakeKind answers fixed outputs and records the requests New receives.
type fakeKind struct {
	name       string
	out, regen genkind.Output
	err        error
	requests   *[]genkind.Request
}

func (k fakeKind) Name() string  { return k.name }
func (k fakeKind) Template() int { return 1 }

func (k fakeKind) New(_ context.Context, _ genkind.Input, req genkind.Request) (genkind.Output, error) {
	if k.requests != nil {
		*k.requests = append(*k.requests, req)
	}
	return k.out, k.err
}

func (k fakeKind) Regenerate(_ context.Context, _ genkind.Input, _ manifest.Entry) (genkind.Output, error) {
	return k.regen, k.err
}

// fakeWirer writes one regenerable file that counts the bindings it got.
type fakeWirer struct{ fakeKind }

func (fakeWirer) Wire(_ context.Context, _ genkind.Input, main string, bindings []genkind.Binding) (genkind.Output, error) {
	path := "cmd/" + main + "/zz_generated_wiring.go"
	content := fmt.Sprintf("%s\n\npackage main\n\n// Wire binds %d values.\nfunc Wire() {}\n", header(genkind.KindWiring, "0.3.0"), len(bindings))
	return genkind.Output{
		Entry:       manifest.Entry{Kind: genkind.KindWiring, Template: "wiring@1", Name: main + ".Wire", Regenerable: []string{path}},
		Regenerable: []genkind.File{{Path: path, Content: []byte(content)}},
	}, nil
}

func wirer() fakeWirer { return fakeWirer{fakeKind{name: genkind.KindWiring}} }

func supportKind() fakeKind {
	out := genkind.Output{
		Entry:       manifest.Entry{Kind: genkind.KindSupport, Template: "support@1", Name: "gensupport.Support", Regenerable: []string{holePath}},
		Regenerable: []genkind.File{{Path: holePath, Content: []byte(header(genkind.KindSupport, "0.3.0") + "\n\npackage gensupport\n")}},
	}
	return fakeKind{name: genkind.KindSupport, out: out, regen: out}
}

func empty() manifest.Manifest { return manifest.Manifest{Schema: manifest.Schema} }

// enrolled is a manifest whose support package is already written.
func enrolled(entries ...manifest.Entry) manifest.Manifest {
	m := empty()
	m.Support = manifest.Support{Path: "internal/gensupport", Template: "support@1"}
	for i := range entries {
		m = m.With(entries[i])
	}
	return m
}

func engineInput(t *testing.T, files map[string][]byte, m manifest.Manifest, kinds ...genkind.Kind) gen.Input {
	t.Helper()
	return gen.Input{
		Module: "example.com/slice", Version: "0.3.0", Policy: model.DefaultGenerate(),
		Snapshot: gentest.Snapshot(t, files), Manifest: m,
		Kinds: kinds, Main: "api",
	}
}

// inSync is the fixture as the last golaw gen left it: the regenerable
// files, the owned file and the manifest as rendered.
func inSync(t *testing.T, m manifest.Manifest) map[string][]byte {
	t.Helper()
	files := gentest.Contracts()
	files[plansGen] = plansGenerated("0.2.0")
	files[plansPath] = []byte(ownedPlans)
	files[holePath] = supportKind().out.Regenerable[0].Content
	encoded, err := m.Encode()
	if err != nil {
		t.Fatal(err)
	}
	files[manifest.Path] = encoded
	return files
}
