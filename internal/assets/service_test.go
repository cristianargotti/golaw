package assets_test

import (
	"bytes"
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
)

// serviceDestinations are the files of the service fixture, in path order.
func serviceDestinations() []string {
	return []string{
		"Dockerfile", "api/generated/server.gen.go", "api/generated/spec.gen.go", "api/generated/types.gen.go",
		"api/openapi.base.yaml", "api/openapi.yaml", "db/migrations/0001_create_items.sql", "db/queries/items.sql",
		"go.mod", "go.sum",
		"internal/adapters/outbound/postgres/sqlcgen/db.go", "internal/adapters/outbound/postgres/sqlcgen/items.sql.go",
		"internal/adapters/outbound/postgres/sqlcgen/models.go",
		"internal/application/catalog.go", "internal/application/catalog_test.go",
		"internal/domain/errors.go", "internal/domain/item.go",
		"internal/errors/problem.go", "internal/errors/problem_test.go",
		"internal/failures/failures.go", "internal/failures/failures_test.go",
		"internal/rbac/rbac.go", "internal/rbac/rbac_test.go",
		"internal/secret/secret.go", "internal/secret/secret_test.go",
		"internal/server/server.go", "internal/server/server_test.go",
		"main.go", "main_internal_test.go",
	}
}

func TestRenderServiceRendersEveryFixtureFile(t *testing.T) {
	t.Parallel()
	files, err := assets.RenderService(golaw.FS, templateValues())
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(files))
	byPath := make(map[string]string, len(files))
	for i := range files {
		paths = append(paths, files[i].Path)
		byPath[files[i].Path] = string(files[i].Data)
		if strings.Contains(byPath[files[i].Path], "{{") {
			t.Errorf("remaining expression in %s", files[i].Path)
		}
	}
	if !slices.Equal(paths, serviceDestinations()) {
		t.Fatalf("service paths = %v", paths)
	}
	module, err := fs.ReadFile(golaw.FS, "templates/examples/service/go.mod.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.NewReplacer("{{.ModulePath}}", "example.test/service", "{{.GoVersion}}", "1.27").Replace(string(module))
	if byPath["go.mod"] != want || !strings.HasPrefix(want, "module example.test/service\n\ngo 1.27\n\nrequire (\n") {
		t.Fatalf("go.mod = %q", byPath["go.mod"])
	}
	if !strings.Contains(byPath["internal/server/server.go"], `"example.test/service/internal/rbac"`) {
		t.Fatal("the server does not import the role policy of the rendered module")
	}
}

func TestRenderServiceRefusesABrokenFixture(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path, body string
		want             error
	}{
		{"red file without the template suffix", "templates/examples/service/README.md", "service\n", assets.ErrManifest},
		{"red unresolved key", "templates/examples/service/a.yml.tmpl", "value: {{.Absent}}\n", assets.ErrRender},
		{"red no fixture", "templates/other.txt", "other\n", assets.ErrManifest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := assets.RenderService(serviceTree(tc.path, tc.body), templateValues()); !errors.Is(err, tc.want) {
				t.Fatalf("RenderService = %v, want %v", err, tc.want)
			}
		})
	}
	near := serviceTree("templates/examples/service/a.yml.tmpl", "module: {{.ModulePath}}\n")
	files, err := assets.RenderService(near, templateValues())
	if err != nil || len(files) != 1 || files[0].Path != "a.yml" ||
		string(files[0].Data) != "module: example.test/service\n" {
		t.Fatalf("near fixture = %+v, %v", files, err)
	}
}

func TestRenderServiceRejectsAnUnlistedFixtureBeforeRendering(t *testing.T) {
	t.Parallel()
	tree := serviceTree("templates/examples/service/a.yml.tmpl", "value: {{.Absent}}\n")
	tree["templates/examples/service/unlisted.yml.tmpl"] = &fstest.MapFile{Data: []byte("value: sample\n")}
	if _, err := assets.RenderService(tree, templateValues()); !errors.Is(err, assets.ErrManifest) {
		t.Fatalf("RenderService = %v, want %v", err, assets.ErrManifest)
	}
}

// TestRenderServiceCarriesTheSumFileByItsName pins the one fixture file
// without the template suffix: a go.sum lists the requirements and never
// names the module, so it holds no placeholder, and the tools that know a
// go.sum by its name, the secret scan among them, must see it as one.
func TestRenderServiceCarriesTheSumFileByItsName(t *testing.T) {
	t.Parallel()
	sum := "example.test/dep v1.0.0 h1:" + strings.Repeat("A", 43) + "=\n"
	files, err := assets.RenderService(serviceTree("templates/examples/service/go.sum", sum), templateValues())
	if err != nil || len(files) != 1 || files[0].Path != "go.sum" || string(files[0].Data) != sum {
		t.Fatalf("a verbatim go.sum = %+v, %v", files, err)
	}
	embedded, err := fs.ReadFile(golaw.FS, "templates/examples/service/go.sum")
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := assets.RenderService(golaw.FS, templateValues())
	if err != nil {
		t.Fatal(err)
	}
	at := slices.IndexFunc(rendered, func(r assets.Rendered) bool { return r.Path == "go.sum" })
	if at < 0 || !bytes.Equal(rendered[at].Data, embedded) {
		t.Fatal("the fixture's go.sum is not the embedded file under its own name")
	}
}

// serviceTree is the smallest embedded tree that holds one asset beside
// the two manifests.
func serviceTree(path, body string) fstest.MapFS {
	names := []string{"agents/manifest.json", path, "templates/manifest.yml"}
	slices.Sort(names)
	return fstest.MapFS{
		"agents/manifest.json":   {Data: []byte(`{"files":["` + strings.Join(names, `","`) + `"]}`)},
		"templates/manifest.yml": {Data: []byte("templates: []\n")},
		path:                     {Data: []byte(body)},
	}
}
