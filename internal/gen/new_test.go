package gen_test

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gentest"
)

var plans = genkind.Request{Name: genkind.Name{Area: "postgres", Type: "Plans"}}

func writeOf(t *testing.T, res gen.Result, path string) gen.Write {
	t.Helper()
	for _, w := range res.Writes {
		if w.Path == path {
			return w
		}
	}
	t.Fatalf("no write of %s in %+v", path, res.Writes)
	return gen.Write{}
}

func TestNewWritesTheKindTheSupportTheWiringAndTheManifest(t *testing.T) {
	t.Parallel()
	sql := fakeKind{name: genkind.KindAdapterSQL, out: plansOutput()}
	res, err := gen.New(t.Context(), engineInput(t, gentest.Contracts(), empty(), supportKind(), sql, wirer()), genkind.KindAdapterSQL, plans)
	if err != nil {
		t.Fatal(err)
	}
	modes := map[string]gen.WriteMode{}
	for _, w := range res.Writes {
		modes[w.Path] = w.Mode
		if w.Base != "" {
			t.Errorf("%s base = %q, want empty for a new file", w.Path, w.Base)
		}
	}
	want := map[string]gen.WriteMode{
		manifest.Path: gen.WriteManifest, wiringPath: gen.WriteRegenerable, plansPath: gen.WriteOwned,
		plansGen: gen.WriteRegenerable, holePath: gen.WriteRegenerable,
	}
	if !reflect.DeepEqual(modes, want) || !slices.IsSortedFunc(res.Writes, func(a, b gen.Write) int { return strings.Compare(a.Path, b.Path) }) {
		t.Fatalf("writes = %v, want %v sorted by path", modes, want)
	}
	m, err := manifest.Decode(writeOf(t, res, manifest.Path).Content)
	if err != nil || m.Support != (manifest.Support{Path: "internal/gensupport", Template: "support@1"}) || len(m.Entries) != 2 ||
		m.Entries[0].Name != "postgres.Plans" || m.Entries[1].Name != "api.Wire" {
		t.Fatalf("manifest = %+v, %v", m, err)
	}
	if !strings.Contains(string(writeOf(t, res, wiringPath).Content), "binds 1 values") {
		t.Errorf("the wiring did not get the adapter's binding")
	}
	open := []holes.Hole{{ID: "plans.read-plan", Constraint: "call p.queries.ReadPlan", Path: plansPath, Line: 7, Func: "ReadPlan"}}
	if !reflect.DeepEqual(res.Open, open) || !reflect.DeepEqual(res.Opened, open) || res.Closed != nil {
		t.Errorf("holes = %+v, opened %+v, closed %+v", res.Open, res.Opened, res.Closed)
	}
	if len(res.Entries) == 0 || res.Entries[0].Name != "postgres.Plans" {
		t.Errorf("entries = %+v", res.Entries)
	}
}

// Red rows: an instance the manifest or the tree already holds, an unknown
// kind and a contract the kind cannot judge.
func TestNewRefusesWhatItWouldOverwrite(t *testing.T) {
	t.Parallel()
	sql := fakeKind{name: genkind.KindAdapterSQL, out: plansOutput()}
	existing := gentest.Contracts()
	existing[plansPath] = []byte(ownedPlans)
	tests := map[string]struct {
		in   gen.Input
		kind string
		want error
		text string
	}{
		"in the manifest":  {engineInput(t, gentest.Contracts(), enrolled(plansEntry()), supportKind(), sql), genkind.KindAdapterSQL, genkind.ErrExists, plansPath},
		"an owned file":    {engineInput(t, existing, empty(), supportKind(), sql), genkind.KindAdapterSQL, genkind.ErrExists, plansPath},
		"an unknown kind":  {engineInput(t, gentest.Contracts(), empty(), supportKind(), sql), "queue", gen.ErrUnknownKind, "queue"},
		"cannot be judged": {engineInput(t, gentest.Contracts(), empty(), supportKind(), fakeKind{name: genkind.KindAdapterSQL, err: genkind.ErrCannotJudge}), genkind.KindAdapterSQL, genkind.ErrCannotJudge, "postgres.Plans"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := gen.New(t.Context(), tc.in, tc.kind, plans); !errors.Is(err, tc.want) || !strings.Contains(err.Error(), tc.text) {
				t.Errorf("New = %v, want %v naming %s", err, tc.want, tc.text)
			}
		})
	}
}

func TestNewRunsTheNeedsOfAKindAndUpdatesAUseCase(t *testing.T) {
	t.Parallel()
	usecaseEntry := manifest.Entry{Kind: genkind.KindUsecase, Template: "usecase@1", Name: "growth.ReadPlan", Ports: []string{"Ledger"}}
	need := genkind.Need{Kind: genkind.KindUsecase, Request: genkind.Request{Name: genkind.Name{Area: "growth", Type: "ReadPlan"}, Ports: []string{"Ledger"}}}
	port := fakeKind{name: genkind.KindPort, out: genkind.Output{
		Entry: manifest.Entry{Kind: genkind.KindPort, Template: "port@1", Name: "growth.Ledger"}, Needs: []genkind.Need{need},
	}}
	ledger := genkind.Request{Name: genkind.Name{Area: "growth", Type: "Ledger"}, Usecase: "ReadPlan"}
	for name, m := range map[string]manifest.Manifest{"a new use case": empty(), "an existing use case": enrolled(usecaseEntry)} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var asked []genkind.Request
			usecase := fakeKind{name: genkind.KindUsecase, out: genkind.Output{Entry: usecaseEntry}, requests: &asked}
			res, err := gen.New(t.Context(), engineInput(t, gentest.Contracts(), m, supportKind(), port, usecase), genkind.KindPort, ledger)
			if err != nil {
				t.Fatal(err)
			}
			if len(asked) != 1 || !reflect.DeepEqual(asked[0], need.Request) || len(res.Entries) != 2 || res.Entries[1].Name != "growth.ReadPlan" {
				t.Errorf("asked = %+v, entries = %+v", asked, res.Entries)
			}
		})
	}
	unknown := port
	unknown.out.Needs = []genkind.Need{{Kind: "queue"}}
	if _, err := gen.New(t.Context(), engineInput(t, gentest.Contracts(), empty(), supportKind(), unknown), genkind.KindPort, ledger); !errors.Is(err, gen.ErrUnknownKind) {
		t.Errorf("an unknown need = %v", err)
	}
}

func TestNewOfTheSupportKindWritesOnlyTheSupportPackage(t *testing.T) {
	t.Parallel()
	res, err := gen.New(t.Context(), engineInput(t, gentest.Contracts(), empty(), supportKind()), genkind.KindSupport, genkind.Request{})
	if err != nil || len(res.Writes) != 2 || res.Writes[0].Path != manifest.Path || res.Writes[1].Path != holePath {
		t.Errorf("New(support) = %+v, %v", res.Writes, err)
	}
}

// An entry whose output carries no binding gives the composition root
// nothing to bind, so New renders none and opens no wiring hole; the near
// row is the adapter with its binding, which TestNewWritesTheKindTheSupport
// TheWiringAndTheManifest proves wired.
func TestNewRendersNoWiringWithoutABinding(t *testing.T) {
	t.Parallel()
	unbound := plansOutput()
	unbound.Bindings = nil
	sql := fakeKind{name: genkind.KindAdapterSQL, out: unbound}
	res, err := gen.New(t.Context(), engineInput(t, gentest.Contracts(), empty(), supportKind(), sql, wirer()), genkind.KindAdapterSQL, plans)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range res.Writes {
		if w.Path == wiringPath {
			t.Errorf("New wrote %s for an entry without a binding", wiringPath)
		}
	}
	m, err := manifest.Decode(writeOf(t, res, manifest.Path).Content)
	if err != nil || len(m.Entries) != 1 {
		t.Errorf("manifest = %+v, %v, want the adapter entry alone", m, err)
	}
}
