package gen_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gentest"
)

func TestNewWiresByHandWhereTheMainIsHandWritten(t *testing.T) {
	t.Parallel()
	sql := fakeKind{name: genkind.KindAdapterSQL, out: plansOutput()}
	in := engineInput(t, gentest.Contracts(), empty(), supportKind(), sql, wirer())
	in.Main = gen.MainByHand
	res, err := gen.New(t.Context(), in, genkind.KindAdapterSQL, plans)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range res.Writes {
		if w.Path == wiringPath {
			t.Errorf("a hand written composition root got %s", w.Path)
		}
	}
	m, err := manifest.Decode(writeOf(t, res, manifest.Path).Content)
	if err != nil || len(m.Entries) != 1 || m.Entries[0].Name != "postgres.Plans" {
		t.Fatalf("manifest = %+v, %v, want the adapter entry alone", m, err)
	}
	var wires []string
	for _, d := range res.Decisions {
		if strings.HasPrefix(d.Text, "wire ") {
			wires = append(wires, d.Text)
		}
	}
	if len(wires) != 1 || !strings.HasSuffix(wires[0], "into the hand written composition root by hand") {
		t.Errorf("decisions = %q, want one wire line per binding", wires)
	}
}

func TestNewWiresAGeneratedMainAsBefore(t *testing.T) {
	t.Parallel()
	sql := fakeKind{name: genkind.KindAdapterSQL, out: plansOutput()}
	res, err := gen.New(t.Context(), engineInput(t, gentest.Contracts(), empty(), supportKind(), sql, wirer()), genkind.KindAdapterSQL, plans)
	if err != nil || writeOf(t, res, wiringPath).Mode != gen.WriteRegenerable {
		t.Fatalf("a generated main = %+v, %v, want its wiring file", res.Writes, err)
	}
	for _, d := range res.Decisions {
		if strings.HasPrefix(d.Text, "wire ") {
			t.Errorf("a generated main got the decision %q", d.Text)
		}
	}
}
