package main

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/thirdparty"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/source"
)

func TestRegenThirdTakesANewSnapshotAfterAGeneratorRan(t *testing.T) {
	t.Parallel()
	files := gentest.Contracts()
	files["golaw.yml"] = append(files["golaw.yml"], []byte("managed:\n  - { path: sqlc.yaml, sha256: "+source.ContentHash(sqlcConfig)+" }\n")...)
	files["sqlc.yaml"] = sqlcConfig
	repo := gentest.Module(t, files)
	h := regenBinding(t)
	warmGenerators(t, h, runnertest.Behavior{Stdout: "ok"})
	snap, err := (binding{}).snapshot(t.Context(), repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	var o regenOutcome
	if got := h.regenThird(t.Context(), repo.Root, snap, nil, &o); got != snap {
		t.Fatal("an event without generators took a new snapshot")
	}
	fresh := h.regenThird(t.Context(), repo.Root, snap, []thirdparty.Generator{sqlcGenerator()}, &o)
	if fresh == snap || len(o.causes) != 0 {
		t.Fatalf("causes = %q, a run kept the old snapshot: %v", o.causes, fresh == snap)
	}
	h.regenThird(t.Context(), t.TempDir(), snap, []thirdparty.Generator{sqlcGenerator()}, &o)
	if len(o.causes) != 1 {
		t.Fatalf("a policy that cannot load gave causes %q", o.causes)
	}
}

func TestRunOneNamesASpentBudgetAndAnUnknownGenerator(t *testing.T) {
	t.Parallel()
	h := regenBinding(t)
	snap := proof.NewTree(map[string][]byte{"sqlc.yaml": sqlcConfig})
	managed := map[string]string{"sqlc.yaml": source.ContentHash(sqlcConfig)}
	spent, cancel := context.WithCancel(t.Context())
	cancel()
	cause, ran := h.runOne(spent, t.TempDir(), managed, snap, sqlcGenerator(), nil)
	if ran || !strings.Contains(cause, "the hook's 3 s budget for external tools was spent") {
		t.Fatalf("cause = %q", cause)
	}
	unknown := thirdparty.Generator{Name: "other", Config: "sqlc.yaml", Hook: true}
	out := h.thirdParty(t.Context(), t.TempDir(), managed, snap, []thirdparty.Generator{unknown})
	if out.ran || !slices.Equal(out.causes, []string{"other is not a generator golaw pins: run golaw gen"}) {
		t.Fatalf("out = %+v", out)
	}
	if got := firstLine([]byte("\n  \n")); got != "no output on stderr" {
		t.Fatalf("first line = %q", got)
	}
}
