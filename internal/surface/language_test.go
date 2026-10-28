package surface_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/surface"
)

func TestLanguagesAreInReportOrder(t *testing.T) {
	t.Parallel()
	want := []surface.Language{
		"javascript", "typescript", "jsx", "tsx", "html", "css", "python", "dockerfile", "cloudformation", "shell", "yaml",
	}
	if got := surface.Languages(); !slices.Equal(got, want) {
		t.Fatalf("Languages() = %v, want %v", got, want)
	}
}

func TestByNameSettlesWhatTheNameDecides(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		path    string
		want    surface.Language
		settled bool
	}{
		"a script":                             {path: "views/charts.js", want: surface.JavaScript, settled: true},
		"an ES module":                         {path: "web/app.mjs", want: surface.JavaScript, settled: true},
		"a CommonJS test":                      {path: "test/view/host.test.cjs", want: surface.JavaScript, settled: true},
		"TypeScript":                           {path: "web/app.ts", want: surface.TypeScript, settled: true},
		"a TypeScript module":                  {path: "web/app.mts", want: surface.TypeScript, settled: true},
		"a CommonJS TypeScript file":           {path: "web/app.cts", want: surface.TypeScript, settled: true},
		"a declaration file":                   {path: "web/types.d.ts", want: surface.TypeScript, settled: true},
		"JSX":                                  {path: "views/panel.jsx", want: surface.JSX, settled: true},
		"TSX":                                  {path: "views/panel.tsx", want: surface.TSX, settled: true},
		"HTML":                                 {path: "views/panel.html", want: surface.HTML, settled: true},
		"an upper case HTML extension":         {path: "views/LEGACY.HTM", want: surface.HTML, settled: true},
		"CSS":                                  {path: "views/site.css", want: surface.CSS, settled: true},
		"Python":                               {path: "test/deploy/test_install.py", want: surface.Python, settled: true},
		"a shell script":                       {path: "scripts/smoke.sh", want: surface.Shell, settled: true},
		"a bash script":                        {path: "scripts/up.bash", want: surface.Shell, settled: true},
		"the Dockerfile":                       {path: "Dockerfile", want: surface.Dockerfile, settled: true},
		"a suffixed Dockerfile":                {path: "deploy/cache.Dockerfile", want: surface.Dockerfile, settled: true},
		"a Dockerfile variant":                 {path: "deploy/Dockerfile.dev", want: surface.Dockerfile, settled: true},
		"a Containerfile":                      {path: "Containerfile", want: surface.Dockerfile, settled: true},
		"a template the content decides":       {path: "infra/10-base.yml", want: "", settled: false},
		"a yaml file the content decides":      {path: "deploy/compose.yaml", want: "", settled: false},
		"a JSON file the content decides":      {path: "infra/stack.json", want: "", settled: false},
		"a .template file the content decides": {path: "infra/stack.template", want: "", settled: false},
		"no extension, a shebang may decide":   {path: "scripts/run", want: "", settled: false},
		"Go is no surface language (near)":     {path: "cmd/hub/main.go", want: "", settled: true},
		"Markdown (near)":                      {path: "README.md", want: "", settled: true},
		"JSON lines (near)":                    {path: "data/rows.jsonl", want: "", settled: true},
		"a dot file (near)":                    {path: "scripts/.bashrc", want: "", settled: true},
		"a Dockerfile ignore file (near)":      {path: "Dockerfile.dockerignore", want: "", settled: true},
		"a Dockerfile template (near)":         {path: "templates/examples/service/Dockerfile.tmpl", want: "", settled: true},
		"another Dockerfile template (near)":   {path: "deploy/Dockerfile.tpl", want: "", settled: true},
		"a properties file (near)":             {path: "sonar-project.properties", want: "", settled: true},
		"a directory named like a file (near)": {path: "views.js/README.md", want: "", settled: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, settled := surface.ByName(tc.path)
			if got != tc.want || settled != tc.settled {
				t.Fatalf("ByName(%q) = %q, %v, want %q, %v", tc.path, got, settled, tc.want, tc.settled)
			}
		})
	}
}
