package gentest_test

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/runner"
)

// The golden slices build offline over the recorded third party outputs of
// C-8, so these tests hold each recording to the exported shapes the pinned
// generator writes for the same contract: a template proven over the
// recording then compiles over the real output.
const (
	sqlcPin = "1.31.1"
	oapiPin = "2.8.0"
)

// pinned answers the generator's name when its pin answers the probe, and
// skips with the reason the acceptance lane records otherwise.
func pinned(t *testing.T, name, pin string, probe ...string) string {
	t.Helper()
	out, err := runner.Run(t.Context(), runner.Command{Name: name, Args: probe})
	if err != nil || out.Code != 0 || !bytes.Contains(append(out.Stdout, out.Stderr...), []byte(pin)) {
		t.Skipf("%s %s not found: run golaw setup --gen", name, pin)
	}
	return name
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	out, err := runner.Run(t.Context(), runner.Command{Name: name, Args: args, Dir: dir})
	if err != nil || out.Code != 0 {
		t.Fatalf("%s %s: %v\n%s%s", name, strings.Join(args, " "), err, out.Stdout, out.Stderr)
	}
}

// fixture writes the C-8 files and one more file under a new directory.
func fixture(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	files := gentest.Contracts()
	files[name] = []byte(content)
	for path, data := range files {
		target := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func goFiles(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("no Go file under %s: %v", dir, err)
	}
	return matches
}

// shapes reads the exported API of Go files: each function and method by
// its signature, each struct by its exported fields, any other type by its
// expression, so an interface is compared by its method set.
func shapes(t *testing.T, paths []string) map[string]string {
	t.Helper()
	out := map[string]string{}
	fset := token.NewFileSet()
	text := func(n ast.Node) string {
		var b bytes.Buffer
		if err := printer.Fprint(&b, fset, n); err != nil {
			t.Fatal(err)
		}
		return strings.Join(strings.Fields(b.String()), " ")
	}
	for _, path := range paths {
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			declShape(out, decl, text)
		}
	}
	return out
}

// declShape records the shape of one declaration in out; the switch lives
// apart from the file loop so each stays under the cognitive limit.
func declShape(out map[string]string, decl ast.Decl, text func(ast.Node) string) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if !d.Name.IsExported() {
			return
		}
		key := "func " + d.Name.Name
		if d.Recv != nil {
			key = "method " + text(d.Recv.List[0].Type) + "." + d.Name.Name
		}
		out[key] = text(d.Type)
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.IsExported() {
				out["type "+ts.Name.Name] = typeShape(ts.Type, text)
			}
		}
	}
}

func typeShape(expr ast.Expr, text func(ast.Node) string) string {
	st, ok := expr.(*ast.StructType)
	if !ok {
		return text(expr)
	}
	var fields []string
	for _, field := range st.Fields.List {
		for _, name := range field.Names {
			if name.IsExported() {
				fields = append(fields, name.Name+" "+text(field.Type))
			}
		}
	}
	return "struct{" + strings.Join(fields, "; ") + "}"
}

// differences names each recorded shape the generator does not write: a
// missing name, another signature, or a recorded field the generated struct
// lacks; a generated name or field the recording leaves out is no
// difference, because the recording holds only what the kinds call.
func differences(recorded, generated map[string]string) []string {
	var out []string
	for _, key := range slices.Sorted(maps.Keys(recorded)) {
		want, ok := generated[key]
		switch {
		case !ok:
			out = append(out, key+": the pinned generator declares no such name")
		case strings.HasPrefix(recorded[key], "struct{"):
			if !fieldsWithin(recorded[key], want) {
				out = append(out, key+": recorded "+recorded[key]+", the generator writes "+want)
			}
		case recorded[key] != want:
			out = append(out, key+": recorded "+recorded[key]+", the generator writes "+want)
		}
	}
	return out
}

func fieldsWithin(recorded, generated string) bool {
	fields := func(s string) []string {
		return strings.Split(strings.TrimSuffix(strings.TrimPrefix(s, "struct{"), "}"), "; ")
	}
	have := fields(generated)
	for _, field := range fields(recorded) {
		if field != "" && !slices.Contains(have, field) {
			return false
		}
	}
	return true
}

func TestDifferencesNameWhatTheRecordingGetsWrong(t *testing.T) {
	t.Parallel()
	const oneArg, twoArgs = "func(ssi StrictServerInterface) http.Handler", "func(ssi StrictServerInterface, middlewares []StrictMiddlewareFunc) ServerInterface"
	tests := map[string]struct {
		recorded, generated map[string]string
		want                int
	}{
		"red: a recorded function the generator does not declare": {recorded: map[string]string{"func NewStrictHandler": oneArg}, generated: map[string]string{}, want: 1},
		"red: a recorded signature the generator changed":         {recorded: map[string]string{"func NewStrictHandler": oneArg}, generated: map[string]string{"func NewStrictHandler": twoArgs}, want: 1},
		"red: a recorded field the generator spells otherwise":    {recorded: map[string]string{"type GetPlanRequestObject": "struct{ID string}"}, generated: map[string]string{"type GetPlanRequestObject": "struct{Id string}"}, want: 1},
		"near: fewer fields than the generator writes": {
			recorded:  map[string]string{"type ServerInterfaceWrapper": "struct{Handler ServerInterface}"},
			generated: map[string]string{"type ServerInterfaceWrapper": "struct{Handler ServerInterface; HandlerMiddlewares []MiddlewareFunc}"},
		},
		"near: a name only the generator declares": {recorded: map[string]string{}, generated: map[string]string{"func HandlerWithOptions": "func(si ServerInterface, options StdHTTPServerOptions) http.Handler"}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := differences(tc.recorded, tc.generated); len(got) != tc.want {
				t.Fatalf("differences = %q, want %d", got, tc.want)
			}
		})
	}
}

func TestTheRecordedServerHasTheShapesOfOapiCodegen(t *testing.T) {
	t.Parallel()
	tool := pinned(t, "oapi-codegen", oapiPin, "-version")
	const config = "package: apigen\ngenerate:\n  models: true\n  std-http-server: true\n  strict-server: true\noutput: real/server.gen.go\n"
	dir := fixture(t, "oapi-codegen.yaml", config)
	if err := os.MkdirAll(filepath.Join(dir, "real"), 0o750); err != nil {
		t.Fatal(err)
	}
	run(t, dir, tool, "-config", "oapi-codegen.yaml", filepath.Join("api", "openapi.yaml"))
	recorded := shapes(t, []string{filepath.Join(dir, "api", "gen", "server.gen.go")})
	if diff := differences(recorded, shapes(t, []string{filepath.Join(dir, "real", "server.gen.go")})); len(diff) > 0 {
		t.Errorf("api/gen/server.gen.go of C-8 drifted from oapi-codegen %s:\n%s", oapiPin, strings.Join(diff, "\n"))
	}
}

func TestTheRecordedQueriesHaveTheShapesOfSqlc(t *testing.T) {
	t.Parallel()
	tool := pinned(t, "sqlc", sqlcPin, "version")
	const config = "version: \"2\"\nsql:\n  - engine: postgresql\n    schema: db/migrations\n    queries: db/queries\n    gen:\n      go:\n        package: sqlcgen\n        out: real/sqlcgen\n        sql_package: database/sql\n"
	dir := fixture(t, "sqlc.yaml", config)
	run(t, dir, tool, "generate", "-f", filepath.Join(dir, "sqlc.yaml"))
	recorded := shapes(t, goFiles(t, filepath.Join(dir, "internal", "adapters", "out", "postgres", "sqlcgen")))
	if diff := differences(recorded, shapes(t, goFiles(t, filepath.Join(dir, "real", "sqlcgen")))); len(diff) > 0 {
		t.Errorf("the sqlcgen files of C-8 drifted from sqlc %s:\n%s", sqlcPin, strings.Join(diff, "\n"))
	}
}
