package handler

import (
	"bytes"
	"fmt"
	"go/parser"
	"go/token"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/goldentest"
	"github.com/cristianargotti/golaw/internal/genheader"
)

// manyOperations is a document of n operations under one tag, for the file
// limit cases.
func manyOperations(n int, tag string) []genkind.Operation {
	ops := make([]genkind.Operation, 0, n)
	for i := range n {
		ops = append(ops, genkind.Operation{
			Name: fmt.Sprintf("ReadItem%03d", i), Tag: tag, Method: http.MethodGet,
			Route: fmt.Sprintf("/items%03d/{id}", i), Line: i + 1,
		})
	}
	return ops
}

func byPath(files []genkind.File) map[string][]byte {
	out := make(map[string][]byte, len(files))
	for _, f := range files {
		out[f.Path] = f.Content
	}
	return out
}

func pathsOf(files []genkind.File) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

// flat folds every run of white space to one blank, so an assertion does not
// depend on the alignment gofmt chose.
func flat(content []byte) string { return strings.Join(strings.Fields(string(content)), " ") }

func requireGo(t *testing.T, files map[string][]byte) {
	t.Helper()
	for path, content := range files {
		if _, err := parser.ParseFile(token.NewFileSet(), path, content, parser.ParseComments); err != nil {
			t.Errorf("%s does not parse: %v\n%s", path, err, content)
		}
		if lines := bytes.Count(content, []byte("\n")); lines > 300 {
			t.Errorf("%s holds %d lines, over the file limit", path, lines)
		}
		for _, shape := range []string{"goverter", "http://", "//" + "nolint", "regexp", "context.Background"} {
			if bytes.Contains(content, []byte(shape)) {
				t.Errorf("%s emits %q", path, shape)
			}
		}
	}
}

func requireLaws(t *testing.T, files map[string][]byte) {
	t.Helper()
	for _, law := range goldentest.Laws() {
		if got := law.Assert(files); len(got) != 0 {
			t.Errorf("law %s: %v", law.Name, got)
		}
	}
}

func TestRegenerableFilesCarryOnlyTheShape(t *testing.T) {
	t.Parallel()
	files, err := regenerable(viewOf(t, operations()), "0.3.0")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		handlerDir + "zz_generated_handler.go", handlerDir + "zz_generated_handler_internal_test.go",
		handlerDir + "zz_generated_handler_routes_test.go", handlerDir + "zz_generated_handler_test.go",
	}
	if got := pathsOf(files); !slices.Equal(got, want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
	all := byPath(files)
	header := genheader.Header{Version: "0.3.0", Kind: genkind.KindHandler, Template: 1}
	for path, content := range all {
		if got, ok := genheader.Parse(content); !ok || got != header {
			t.Errorf("%s header = %+v, %v, want %q on the first line", path, got, ok, header.Line())
		}
	}
	requireGo(t, all)
	requireLaws(t, all)
	tests := map[string][]string{
		handlerDir + "zz_generated_handler.go": {
			"type Handler struct { uc growth.UseCases }",
			"func NewHandler(uc growth.UseCases) *Handler { return &Handler{uc: uc} }",
			"var _ apigen.StrictServerInterface = (*Handler)(nil)",
			"func Mount(mux *http.ServeMux, h *Handler) { apigen.HandlerFromMux(apigen.NewStrictHandler(h, nil), mux) }",
			"func statusOf(err error, rules ...statusRule) int {",
			"if errors.Is(err, rule.err) { return rule.status }",
		},
		handlerDir + "zz_generated_handler_test.go": {
			"package plansapi_test",
			"func newTestHandler() *plansapi.Handler {",
			"growth.NewUseCases(growth.Deps{ Clock: clocktest.NewFake(), Ledger: ledgertest.NewFake(), })",
			"func sampleRequest(t *testing.T, pattern string) *http.Request { t.Helper()",
			"httptest.NewRequestWithContext(t.Context(), method, strings.Join(segments, \"/\"), http.NoBody)",
			"func requireRoutes(t *testing.T, patterns []string) { t.Helper()",
			"func requireCauses(t *testing.T, operations map[string]func(context.Context) error) {",
			"!errors.Is(err, gensupport.ErrNotImplemented) && !wrapped(err)",
			"func filled[Request any]() Request {",
			"target.Set(reflect.New(field.Type.Elem()))",
		},
		handlerDir + "zz_generated_handler_routes_test.go": {
			"func TestMountRoutes(t *testing.T) { t.Parallel()",
			`requireRoutes(t, []string{ "GET /plans/{id}", "POST /plans/{id}/archive", })`,
			"func TestOperationsKeepTheCauseOfAnError(t *testing.T) {",
			`requireCauses(t, map[string]func(context.Context) error{ "GetPlan": call(h.GetPlan), "ArchivePlan": call(h.ArchivePlan), })`,
		},
		handlerDir + "zz_generated_handler_internal_test.go": {
			"package plansapi",
			"func TestStatusOfMapsEveryAnswer(t *testing.T) {",
			"statusOf(tc.err, rules...)",
		},
	}
	for path, wants := range tests {
		for _, want := range wants {
			if !strings.Contains(flat(all[path]), want) {
				t.Errorf("%s lacks %q:\n%s", path, want, all[path])
			}
		}
	}
}

func TestRegenerableFilesNameEveryArea(t *testing.T) {
	t.Parallel()
	in := input(t)
	in.Manifest = withBilling(in.Manifest)
	v, err := build(in, "plansapi", "api/openapi.yaml", "api/gen/server.gen.go", []string{"growth", "billing"})
	if err != nil {
		t.Fatal(err)
	}
	files, err := regenerable(v, "0.3.0")
	if err != nil {
		t.Fatal(err)
	}
	all := byPath(files)
	requireGo(t, all)
	requireLaws(t, all)
	for path, want := range map[string]string{
		handlerDir + "zz_generated_handler.go":      "func NewHandler(ucBilling billing.UseCases, ucGrowth growth.UseCases) *Handler { return &Handler{ucBilling: ucBilling, ucGrowth: ucGrowth} }",
		handlerDir + "zz_generated_handler_test.go": `growthledgertest "example.com/slice/internal/application/growth/ledgertest"`,
	} {
		if !strings.Contains(flat(all[path]), want) {
			t.Errorf("%s lacks %q:\n%s", path, want, all[path])
		}
	}
}

func TestRegenerableRouteTableSplitsAtTheFileLimit(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		ops    int
		chunks int
	}{
		"red: one operation over the limit opens a second route file": {ops: routesPerFile + 1, chunks: 2},
		"near: the limit itself stays in one file":                    {ops: routesPerFile, chunks: 1},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			files, err := regenerable(viewOf(t, manyOperations(tc.ops, "items")), "0.3.0")
			if err != nil {
				t.Fatal(err)
			}
			all := byPath(files)
			requireGo(t, all)
			second, split := all[handlerDir+"zz_generated_handler_routes_2_test.go"]
			if split != (tc.chunks == 2) {
				t.Fatalf("second route file present %v, want %v: %v", split, tc.chunks == 2, pathsOf(files))
			}
			first := strings.Count(string(all[handlerDir+"zz_generated_handler_routes_test.go"]), `"GET /items`)
			if first != min(tc.ops, routesPerFile) || (split && !strings.Contains(flat(second), "func TestMountRoutes2(t *testing.T) {")) {
				t.Errorf("first route file holds %d patterns, second:\n%s", first, second)
			}
		})
	}
}

func TestRegenerableFilesAreDeterministic(t *testing.T) {
	t.Parallel()
	v := viewOf(t, operations())
	first, err := regenerable(v, "0.3.0")
	if err != nil {
		t.Fatal(err)
	}
	again, err := regenerable(v, "0.3.0")
	if err != nil {
		t.Fatal(err)
	}
	for i := range first {
		if first[i].Path != again[i].Path || !bytes.Equal(first[i].Content, again[i].Content) || bytes.Contains(first[i].Content, []byte("\r")) {
			t.Errorf("%s differs between two renderings or carries a carriage return", first[i].Path)
		}
	}
}

func TestRegenerableHeaderNamesTheKitVersion(t *testing.T) {
	t.Parallel()
	files, err := regenerable(viewOf(t, operations()), "1.4.2")
	if err != nil {
		t.Fatal(err)
	}
	want := genheader.Header{Version: "1.4.2", Kind: genkind.KindHandler, Template: 1}
	for _, f := range files {
		if got, ok := genheader.Parse(f.Content); !ok || got != want {
			t.Errorf("%s header = %+v, %v, want %q", f.Path, got, ok, want.Line())
		}
	}
}

func TestChunkNeverAnswersAnEmptyRun(t *testing.T) {
	t.Parallel()
	if got := chunk([]int{}, routesPerFile); got != nil {
		t.Errorf("chunk of no item = %v, want nil", got)
	}
	if got := chunk([]int{1, 2, 3}, 2); len(got) != 2 || len(got[1]) != 1 {
		t.Errorf("chunk of three by two = %v", got)
	}
}
