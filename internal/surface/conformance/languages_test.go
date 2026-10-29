// Package conformance_test runs the surface model over the fixture
// repository of the hub issue classes, from outside the surface package,
// the way every later consumer reads it.
package conformance_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/surfacefix"
)

// byName groups the paths of a twin by the language their name settles;
// the paths the content decides are listed under the empty language.
func byName(t *testing.T, twin surfacefix.Twin) map[surface.Language][]string {
	t.Helper()
	files, err := surfacefix.Files(twin)
	if err != nil {
		t.Fatal(err)
	}
	out := map[surface.Language][]string{}
	for _, name := range slices.Sorted(maps.Keys(files)) {
		language, settled := surface.ByName(name)
		switch {
		case !settled:
			out[""] = append(out[""], name)
		case language != "":
			out[language] = append(out[language], name)
		}
	}
	return out
}

func TestTheTwinsNameTheLanguagesOfTheHubClasses(t *testing.T) {
	t.Parallel()
	python := []string{"deploy/__init__.py", "deploy/install.py", "test/__init__.py", "test/deploy/__init__.py", "test/deploy/test_install.py"}
	common := map[surface.Language][]string{
		"":                 {"infra/10-base.yml"},
		surface.JSX:        {"views/panel.jsx"},
		surface.HTML:       {"test/view/page.html", "views/panel.html"},
		surface.Python:     python,
		surface.Dockerfile: {"Dockerfile", "deploy/cache.Dockerfile"},
		surface.Shell:      {"scripts/smoke.sh"},
	}
	red, corrected := maps.Clone(common), maps.Clone(common)
	red[surface.JavaScript] = []string{"test/view/host.test.cjs", "views/charts.js"}
	corrected[surface.JavaScript] = []string{"test/view/host.test.cjs", "views/charts.js", "views/panel.js", "views/render.cjs"}
	for name, tc := range map[string]struct {
		twin surfacefix.Twin
		want map[surface.Language][]string
	}{
		"the red twin":       {twin: surfacefix.Red, want: red},
		"the corrected twin": {twin: surfacefix.Corrected, want: corrected},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := byName(t, tc.twin); !maps.EqualFunc(got, tc.want, slices.Equal) {
				t.Fatalf("languages by name = %v, want %v", got, tc.want)
			}
		})
	}
}
