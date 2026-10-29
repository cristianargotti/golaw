package surface_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/surface"
)

func TestOutsideShapeNamesAFullProjectAtTheRoot(t *testing.T) {
	t.Parallel()
	const (
		node   = "package.json declares a build script: a Node or TypeScript project is outside M6, which judges the files it reads"
		python = "pyproject.toml declares a build backend: a Python package is outside M6, which judges the files it reads"
	)
	tests := map[string]struct {
		files map[string]string
		want  string
	}{
		"a Node project":    {files: map[string]string{"package.json": `{"scripts":{"build":"next build","lint":"eslint ."}}`}, want: node},
		"a Python package":  {files: map[string]string{"pyproject.toml": "[build-system]\nrequires = [\"hatchling\"]\nbuild-backend = \"hatchling.build\"\n"}, want: python},
		"a commented table": {files: map[string]string{"pyproject.toml": "[project]\nname = \"x\"\n[ build-system ] # packaging\nbuild-backend = 'flit_core.buildapi'\n"}, want: python},
		"a dotted key":      {files: map[string]string{"pyproject.toml": "build-system.build-backend = \"x\"\n"}, want: python},
		"a quoted key":      {files: map[string]string{"pyproject.toml": "[build-system]\n\"build-backend\" = \"x\" # the backend\n"}, want: python},
		"both, Node first": {
			files: map[string]string{"package.json": `{"scripts":{"build":"tsc"}}`, "pyproject.toml": "[build-system]\nbuild-backend = \"x\"\n"},
			want:  node,
		},
		"test scripts only (near)":              {files: map[string]string{"package.json": `{"scripts":{"test":"node --test test/"}}`}},
		"no scripts (near)":                     {files: map[string]string{"package.json": `{"name":"views","private":true}`}},
		"an unreadable manifest (near)":         {files: map[string]string{"package.json": `{"scripts":`}},
		"a manifest below the root (near)":      {files: map[string]string{"web/package.json": `{"scripts":{"build":"vite build"}}`}},
		"a tool table only (near)":              {files: map[string]string{"pyproject.toml": "[tool.ruff]\nline-length = 100\n"}},
		"requires without a backend (near)":     {files: map[string]string{"pyproject.toml": "[build-system]\nrequires = [\"setuptools\"]\n"}},
		"a backend key in another table (near)": {files: map[string]string{"pyproject.toml": "[tool.poetry]\nbuild-backend = \"x\"\n"}},
		"a commented backend (near)":            {files: map[string]string{"pyproject.toml": "[build-system]\n# build-backend = \"x\"\n"}},
		"no manifest (near)":                    {files: map[string]string{"go.mod": "module example.test/m\n"}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, outside := surface.OutsideShape(snapshotOf(tc.files))
			if got != tc.want || outside != (tc.want != "") {
				t.Fatalf("OutsideShape = %q, %v, want %q", got, outside, tc.want)
			}
		})
	}
}
