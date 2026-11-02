package wiring_test

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/goldentest"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/kinds/wiring"
	"github.com/cristianargotti/golaw/internal/genheader"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/model"
)

const slice = "example.com/slice"

func bindings() []genkind.Binding {
	return []genkind.Binding{
		{Kind: genkind.KindHandler, Area: "growth", Import: genkind.Import{Path: slice + "/internal/adapters/in/plansapi"}, Call: "plansapi.NewHandler"},
		{
			Kind: genkind.KindAdapterHTTP, Area: "growth", Port: "Clock", Import: genkind.Import{Path: slice + "/internal/adapters/out/clockhttp"},
			Call:  "clockhttp.NewClient(clockhttp.Config{Client: infra.HTTP})",
			Needs: []genkind.InfraUse{{Field: "HTTP", Type: "*http.Client", Import: genkind.Import{Path: "net/http"}}},
		},
		{
			Kind: genkind.KindAdapterSQL, Area: "growth", Port: "Ledger", Import: genkind.Import{Path: slice + "/internal/adapters/out/postgres"},
			Call:  "postgres.NewPlans(infra.DB)",
			Needs: []genkind.InfraUse{{Field: "DB", Type: "*sql.DB", Import: genkind.Import{Path: "database/sql"}}},
		},
		{Kind: genkind.KindUsecase, Area: "growth", Import: genkind.Import{Path: slice + "/internal/application/growth"}, Call: "growth.NewUseCases"},
	}
}

func wireInput(t *testing.T, files map[string][]byte) genkind.Input {
	t.Helper()
	return genkind.Input{Module: slice, Version: "0.3.0", Policy: model.DefaultGenerate(), Snapshot: gentest.Snapshot(t, files)}
}

// squash compares rendered Go without the alignment gofmt adds.
func squash(b []byte) string { return strings.Join(strings.Fields(string(b)), " ") }

func paths(files []genkind.File) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

func TestWireComposesTheAdaptersTheUseCasesAndTheHandlers(t *testing.T) {
	t.Parallel()
	out, err := wiring.Kind().Wire(t.Context(), wireInput(t, gentest.Contracts()), "api", bindings())
	if err != nil {
		t.Fatal(err)
	}
	regenerable := []string{"cmd/api/zz_generated_wiring.go", "cmd/api/zz_generated_wiring_internal_test.go"}
	owned := []string{"cmd/api/main.go", "cmd/api/wiring.go", "cmd/api/wiring_internal_test.go"}
	if !slices.Equal(paths(out.Regenerable), regenerable) || !slices.Equal(paths(out.Owned), owned) {
		t.Fatalf("regenerable %v, owned %v", paths(out.Regenerable), paths(out.Owned))
	}
	e := out.Entry
	if e.Kind != genkind.KindWiring || e.Template != "wiring@1" || e.Name != "api.Wire" || !slices.Equal(e.Regenerable, regenerable) || !slices.Equal(e.Owned, owned) {
		t.Fatalf("entry = %+v", e)
	}
	if h, ok := genheader.Parse(out.Regenerable[0].Content); !ok || h.Kind != genkind.KindWiring {
		t.Errorf("header = %+v, %v", h, ok)
	}
	wire := squash(out.Regenerable[0].Content)
	for _, want := range []string{
		"type Infra struct { DB *sql.DB HTTP *http.Client }",
		"func Wire(infra Infra) *http.ServeMux { mux := http.NewServeMux() growthUC := wireGrowth(infra) plansapi.Mount(mux, plansapi.NewHandler(growthUC)) return mux }",
		"func wireGrowth(infra Infra) growth.UseCases { return growth.NewUseCases(growth.Deps{ Clock: clockhttp.NewClient(clockhttp.Config{Client: infra.HTTP}), Ledger: postgres.NewPlans(infra.DB), }) }",
		`"database/sql"`, `"net/http"`, `"example.com/slice/internal/application/growth"`,
		"func Run(ctx context.Context, serve func(context.Context) error) int {",
	} {
		if !strings.Contains(wire, want) {
			t.Errorf("wiring lacks %q:\n%s", want, out.Regenerable[0].Content)
		}
	}
	requireRun(t, out)
	found, err := holes.Find(owned[1], out.Owned[1].Content)
	if err != nil || len(found) != 1 || found[0].ID != "wiring.infra" || found[0].Func != "Serve" {
		t.Errorf("owned holes = %+v, %v", found, err)
	}
	reversed := bindings()
	slices.Reverse(reversed)
	again, err := wiring.Kind().Wire(t.Context(), wireInput(t, gentest.Contracts()), "api", reversed)
	if err != nil || !bytes.Equal(again.Regenerable[0].Content, out.Regenerable[0].Content) {
		t.Errorf("the order of the bindings changed the bytes: %v", err)
	}
}

func TestWireFollowsTheLayoutAndTheBindings(t *testing.T) {
	t.Parallel()
	noHandler := bindings()[1:]
	noInfra := []genkind.Binding{{Kind: genkind.KindUsecase, Area: "growth", Import: genkind.Import{Path: slice + "/internal/application/growth"}, Call: "growth.NewUseCases"}}
	withMain := gentest.Contracts()
	withMain["cmd/api/server.go"] = []byte("package main\n\nfunc main() {}\n")
	tests := map[string]struct {
		files    map[string][]byte
		main     string
		bindings []genkind.Binding
		check    func(t *testing.T, out genkind.Output)
	}{
		"the root layout":                            {gentest.Contracts(), "", bindings(), checkRootLayout},
		"near: a main package that declares main":    {withMain, "api", bindings(), checkNoMainBesideMain},
		"an area no handler takes":                   {gentest.Contracts(), "api", noHandler, checkUnusedArea},
		"no infrastructure value":                    {gentest.Contracts(), "api", noInfra, checkNoInfra},
		"red: an adapter whose area has no use case": {gentest.Contracts(), "api", bindings()[1:3], checkAdapterWithoutUseCase},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out, err := wiring.Kind().Wire(t.Context(), wireInput(t, tc.files), tc.main, tc.bindings)
			if err != nil {
				t.Fatal(err)
			}
			tc.check(t, out)
		})
	}
}

// The checks of TestWireFollowsTheLayoutAndTheBindings, named so each row
// marks itself a helper and the table keeps its cognitive complexity low.

func checkRootLayout(t *testing.T, out genkind.Output) {
	t.Helper()
	if out.Entry.Name != "main.Wire" || out.Regenerable[0].Path != "zz_generated_wiring.go" {
		t.Errorf("root entry = %+v", out.Entry)
	}
}

func checkNoMainBesideMain(t *testing.T, out genkind.Output) {
	t.Helper()
	if slices.Contains(paths(out.Owned), "cmd/api/main.go") || slices.Contains(out.Entry.Owned, "cmd/api/main.go") {
		t.Errorf("main.go rendered beside a main function: %v", paths(out.Owned))
	}
}

func checkUnusedArea(t *testing.T, out genkind.Output) {
	t.Helper()
	if !strings.Contains(squash(out.Regenerable[0].Content), "_ = wireGrowth(infra)") {
		t.Errorf("an unused area is not kept compiling:\n%s", out.Regenerable[0].Content)
	}
}

func checkNoInfra(t *testing.T, out genkind.Output) {
	t.Helper()
	wire := squash(out.Regenerable[0].Content)
	if !strings.Contains(wire, "func Wire(_ Infra) *http.ServeMux") || !strings.Contains(wire, "func wireGrowth() growth.UseCases") {
		t.Errorf("an unused infra parameter:\n%s", out.Regenerable[0].Content)
	}
}

func checkAdapterWithoutUseCase(t *testing.T, out genkind.Output) {
	t.Helper()
	if len(out.Decisions) != 2 || !strings.Contains(out.Decisions[0].Text, "which no use case consumes") {
		t.Errorf("decisions = %+v", out.Decisions)
	}
}

// Red rows: one infrastructure field of two types, and the kind entries no
// command runs.
func TestWireRefusesWhatItCannotCompose(t *testing.T) {
	t.Parallel()
	conflict := append(bindings(), genkind.Binding{
		Kind: genkind.KindAdapterSQL, Area: "growth", Port: "Archive", Call: "archive.New(infra.DB)",
		Needs: []genkind.InfraUse{{Field: "DB", Type: "*pgxpool.Pool", Import: genkind.Import{Path: "github.com/acme/pgxpool"}}},
	})
	if _, err := wiring.Kind().Wire(t.Context(), wireInput(t, gentest.Contracts()), "api", conflict); !errors.Is(err, genkind.ErrRequest) || !strings.Contains(err.Error(), "DB") {
		t.Errorf("a field of two types = %v", err)
	}
	k := wiring.Kind()
	if _, err := k.New(t.Context(), wireInput(t, nil), genkind.Request{}); !errors.Is(err, genkind.ErrRequest) {
		t.Errorf("New = %v", err)
	}
	if _, err := k.Regenerate(t.Context(), wireInput(t, nil), genkind.Output{}.Entry); !errors.Is(err, genkind.ErrRequest) {
		t.Errorf("Regenerate = %v", err)
	}
	if k.Name() != genkind.KindWiring || k.Template() != 1 {
		t.Errorf("kind = %s@%d", k.Name(), k.Template())
	}
}

func TestWiringShapesKeepTheTemplateLaws(t *testing.T) {
	t.Parallel()
	goldentest.Prove(t, "testdata/proof.yml", goldentest.Laws())
}

// requireRun holds main.go to one call of Run and the wiring suite to its
// test, so the root's statements stay in the generated suite.
func requireRun(t *testing.T, out genkind.Output) {
	t.Helper()
	if main := squash(out.Owned[0].Content); !strings.Contains(main, "func main() { os.Exit(Run(context.Background(), Serve)) }") {
		t.Errorf("main.go does more than call Run:\n%s", out.Owned[0].Content)
	}
	if test := squash(out.Regenerable[1].Content); !strings.Contains(test, "func TestRunAnswersTheOutcomeOfServe(t *testing.T) {") {
		t.Errorf("the wiring suite does not test Run:\n%s", out.Regenerable[1].Content)
	}
}

// Red rows: a file of the main package already declares a name the root
// golaw writes declares, so the package would stop compiling after the
// next golaw new; the refusal names the file and the name. Near rows: the
// root's own files, a method of that name and a test file never collide.
func TestWireRefusesANameTheMainPackageAlreadyDeclares(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		path, source, want string
	}{
		"red: Run in another file":            {"cmd/api/run.go", "package main\n\nfunc Run() {}\n", "cmd/api/run.go declares Run"},
		"red: Wire in another file":           {"cmd/api/routes.go", "package main\n\nfunc Wire() {}\n", "cmd/api/routes.go declares Wire"},
		"red: the type Infra in another file": {"cmd/api/infra.go", "package main\n\ntype Infra struct{}\n", "cmd/api/infra.go declares Infra"},
		"red: Serve beside wiring.go":         {"cmd/api/serve.go", "package main\n\nfunc Serve() {}\n", "cmd/api/serve.go declares Serve"},
		"near: Serve in the owned wiring.go":  {"cmd/api/wiring.go", "package main\n\nfunc Serve() {}\n", ""},
		"near: a method named Run":            {"cmd/api/job.go", "package main\n\ntype job struct{}\n\nfunc (job) Run() {}\n", ""},
		"near: Run in a test file":            {"cmd/api/run_test.go", "package main\n\nfunc Run() {}\n", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			files := gentest.Contracts()
			files[tc.path] = []byte(tc.source)
			_, err := wiring.Kind().Wire(t.Context(), wireInput(t, files), "api", bindings())
			switch {
			case tc.want == "" && err != nil:
				t.Fatalf("Wire = %v, want no refusal", err)
			case tc.want != "" && (!errors.Is(err, genkind.ErrRequest) || !strings.Contains(fmt.Sprint(err), tc.want)):
				t.Fatalf("Wire = %v, want a refusal naming %q", err, tc.want)
			}
		})
	}
}
