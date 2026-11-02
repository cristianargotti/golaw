package wiring_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/wiring"
	"github.com/cristianargotti/golaw/internal/gentest"
)

// The rows beside the plan's: the edges of a binding set the engine can
// hand the kind, each proven by the bytes or the decision it yields.
func TestWireComposesTheEdgesOfTheBindings(t *testing.T) {
	t.Parallel()
	usecase := func(area string) genkind.Binding {
		return genkind.Binding{Kind: genkind.KindUsecase, Area: area, Import: genkind.Import{Path: slice + "/internal/application/" + area}, Call: area + ".NewUseCases"}
	}
	named := genkind.Binding{Kind: genkind.KindHandler, Area: "growth", Import: genkind.Import{Name: "api", Path: slice + "/internal/adapters/in/plansapi"}, Call: "api.NewHandler"}
	shared := []genkind.Binding{
		usecase("growth"), usecase("billing"), usecase("growth"),
		{Kind: genkind.KindHandler, Area: "billing", Import: genkind.Import{Path: slice + "/internal/adapters/in/plansapi"}, Call: "plansapi.NewHandler"},
		{Kind: genkind.KindHandler, Area: "growth", Import: genkind.Import{Path: slice + "/internal/adapters/in/plansapi"}, Call: "plansapi.NewHandler"},
		{Kind: genkind.KindHandler, Area: "audit", Import: genkind.Import{Path: slice + "/internal/adapters/in/auditapi"}, Call: "auditapi.NewHandler"},
	}
	tests := map[string]struct {
		bindings []genkind.Binding
		want     []string
		lacks    []string
		decision string
	}{
		"a named handler import": {
			bindings: []genkind.Binding{usecase("growth"), named},
			want:     []string{`api "example.com/slice/internal/adapters/in/plansapi"`, "api.Mount(mux, api.NewHandler(growthUC))"},
		},
		"one handler bound twice to one area": {
			bindings: append(bindings(), bindings()[0]),
			want:     []string{"plansapi.Mount(mux, plansapi.NewHandler(growthUC)) return mux"},
		},
		"one handler over two areas and one without a use case": {
			bindings: shared,
			want:     []string{"plansapi.Mount(mux, plansapi.NewHandler(billingUC, growthUC))", "func wireBilling() billing.UseCases"},
			lacks:    []string{"auditapi", "wireAudit"},
			decision: "auditapi.NewHandler takes area audit, which has no use case",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out, err := wiring.Kind().Wire(t.Context(), wireInput(t, gentest.Contracts()), "api", tc.bindings)
			if err != nil {
				t.Fatal(err)
			}
			assertWire(t, out.Regenerable[0].Content, tc.want, tc.lacks)
			if tc.decision != "" && (len(out.Decisions) != 1 || !strings.Contains(out.Decisions[0].Text, tc.decision) || out.Decisions[0].Path != "cmd/api/zz_generated_wiring.go") {
				t.Errorf("decisions = %+v", out.Decisions)
			}
		})
	}
}

// assertWire checks the rendered root holds every want and no lack.
func assertWire(t *testing.T, content []byte, want, lacks []string) {
	t.Helper()
	wire := squash(content)
	for _, w := range want {
		if !strings.Contains(wire, w) {
			t.Errorf("wiring lacks %q:\n%s", w, content)
		}
	}
	for _, lack := range lacks {
		if strings.Contains(wire, lack) {
			t.Errorf("wiring holds %q:\n%s", lack, content)
		}
	}
}

func TestWireRefusesAPortBoundTwiceAndADoneContext(t *testing.T) {
	t.Parallel()
	twice := append(bindings(), genkind.Binding{
		Kind: genkind.KindAdapterSQL, Area: "growth", Port: "Ledger", Import: genkind.Import{Path: slice + "/internal/adapters/out/mysql"}, Call: "mysql.NewPlans(infra.DB)",
	})
	if _, err := wiring.Kind().Wire(t.Context(), wireInput(t, gentest.Contracts()), "api", twice); !errors.Is(err, genkind.ErrRequest) || !strings.Contains(err.Error(), "Ledger") {
		t.Errorf("a port bound twice = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := wiring.Kind().Wire(ctx, wireInput(t, gentest.Contracts()), "api", bindings()); !errors.Is(err, context.Canceled) {
		t.Errorf("a done context = %v", err)
	}
}

func TestWireKeepsTheEntryOfAnExistingMainAndAMovedSupport(t *testing.T) {
	t.Parallel()
	files := gentest.Contracts()
	files["main.go"] = []byte("package main\n\nfunc main() {}\n")
	files["broken.go"] = []byte("package main\n\nfunc (\n")
	files["other/main.go"] = []byte("package main\n\nfunc main() {}\n")
	in := wireInput(t, files)
	in.Policy.Support = "internal/platform/support"
	out, err := wiring.Kind().Wire(t.Context(), in, "", bindings())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"main.go", "wiring.go", "wiring_internal_test.go"}; !slices.Equal(out.Entry.Owned, want) || slices.Contains(paths(out.Owned), "main.go") {
		t.Errorf("entry owned %v, rendered %v", out.Entry.Owned, paths(out.Owned))
	}
	if !strings.Contains(string(out.Owned[0].Content), `gensupport "example.com/slice/internal/platform/support"`) {
		t.Errorf("the moved support import is not named:\n%s", out.Owned[0].Content)
	}
	fresh, err := wiring.Kind().Wire(t.Context(), wireInput(t, gentest.Contracts()), "", bindings())
	if err != nil || !strings.Contains(string(fresh.Owned[0].Content), "// Command slice serves the routes") {
		t.Errorf("root main.go = %v:\n%s", err, fresh.Owned[0].Content)
	}
}
