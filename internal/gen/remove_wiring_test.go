package gen_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
)

// A removed entry is never wired: the composition root that bound it is
// rendered again without it, even when it was the last entry, so the root
// never names code the person deleted.
func TestARemovedEntryLeavesTheCompositionRoot(t *testing.T) {
	t.Parallel()
	wiring := manifest.Entry{
		Kind: genkind.KindWiring, Template: "wiring@1", Name: "api.Wire",
		Regenerable: []string{wiringPath}, Owned: []string{"cmd/api/main.go"},
	}
	m := enrolled(plansEntry(), wiring)
	files := inSync(t, m)
	files["cmd/api/main.go"] = []byte("package main\n")
	files[wiringPath] = []byte(header(genkind.KindWiring, "0.3.0") + "\n\npackage main\n\n// Wire binds 1 values.\nfunc Wire() {}\n")
	delete(files, plansPath)
	res, err := gen.Generate(t.Context(), engineInput(t, files, m, supportKind(), regenerating(), wirer()))
	if err != nil || len(res.Removed) != 1 {
		t.Fatalf("removed = %+v, %v", res.Removed, err)
	}
	if w := writeOf(t, res, wiringPath); !strings.Contains(string(w.Content), "Wire binds 0 values") {
		t.Errorf("the root = %q, want it rendered without the removed binding", w.Content)
	}
	if _, kept := res.Manifest.Lookup(genkind.KindWiring, "api.Wire"); !kept {
		t.Error("the wiring entry left the manifest: its owned main stays the person's")
	}
}
