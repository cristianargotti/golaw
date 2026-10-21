package assets_test

import (
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// floorSets are every capability set a valid policy can hold: every subset
// of the capabilities that keeps the floor of policy.CheckCapabilities.
func floorSets() [][]string {
	all := model.Capabilities()
	var sets [][]string
	for mask := range 1 << len(all) {
		set := []string{}
		for i, name := range all {
			if mask&(1<<i) != 0 {
				set = append(set, name)
			}
		}
		if policy.CheckCapabilities(set) == nil {
			sets = append(sets, set)
		}
	}
	return sets
}

// renderedSources are the template sources a render under the capabilities
// selects, in the order of their destinations.
func renderedSources(t *testing.T, source fs.FS, capabilities []string) []string {
	t.Helper()
	context := assetContext()
	context.Capabilities = capabilities
	files, err := assets.RenderAllContext(source, context)
	if err != nil {
		t.Fatalf("render under %v: %v", capabilities, err)
	}
	sources := make([]string, 0, len(files))
	for i := range files {
		sources = append(sources, files[i].Template.Source)
	}
	slices.Sort(sources)
	return sources
}

// conditioned are the sources of the manifest whose condition holds under
// the capabilities.
func conditioned(entries []assets.Template, capabilities []string) []string {
	var sources []string
	for _, entry := range entries {
		if !slices.ContainsFunc(entry.When, func(c string) bool { return !slices.Contains(capabilities, c) }) {
			sources = append(sources, entry.Source)
		}
	}
	slices.Sort(sources)
	return sources
}

// TestEveryTemplateRendersOnlyWhereItsConditionHolds reads the manifest as
// every part leaves it: under every capability set a valid policy holds, a
// template renders exactly when every capability its when names holds.
func TestEveryTemplateRendersOnlyWhereItsConditionHolds(t *testing.T) {
	t.Parallel()
	entries, err := assets.Templates(golaw.FS)
	if err != nil {
		t.Fatal(err)
	}
	sets := floorSets()
	if len(sets) != 34 || len(sets[0]) != 0 || !slices.Equal(sets[len(sets)-1], model.Capabilities()) {
		t.Fatalf("capability sets = %d, first %v, last %v", len(sets), sets[0], sets[len(sets)-1])
	}
	for _, set := range sets {
		if got, want := renderedSources(t, golaw.FS, set), conditioned(entries, set); !slices.Equal(got, want) {
			t.Errorf("under %v rendered %v, the conditions give %v", set, got, want)
		}
	}
}

// conditionTree is a manifest of three templates: one for every repository,
// one under db and one under api with go.
func conditionTree() fstest.MapFS {
	entry := func(name, when string) string {
		return "  - source: templates/" + name + ".yml.tmpl\n    destination: " + name + ".yml\n    ownership: kit\n    when: " + when + "\n"
	}
	return fstest.MapFS{
		"templates/manifest.yml":  {Data: []byte("templates:\n" + entry("api", "[api, go]") + entry("core", "[]") + entry("db", "[db]"))},
		"templates/api.yml.tmpl":  {Data: []byte("api: true\n")},
		"templates/core.yml.tmpl": {Data: []byte("core: true\n")},
		"templates/db.yml.tmpl":   {Data: []byte("db: true\n")},
	}
}

func TestAConditionSelectsItsTemplate(t *testing.T) {
	t.Parallel()
	core, db, api := "templates/core.yml.tmpl", "templates/db.yml.tmpl", "templates/api.yml.tmpl"
	cases := map[string]struct {
		capabilities []string
		want         []string
	}{
		"none":        {[]string{}, []string{core}},
		"go":          {[]string{"go"}, []string{core}},
		"db with go":  {[]string{"db", "go"}, []string{core, db}},
		"api with go": {[]string{"api", "go"}, []string{api, core}},
		"every one":   {model.Capabilities(), []string{api, core, db}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := renderedSources(t, conditionTree(), tc.capabilities); !slices.Equal(got, tc.want) {
				t.Fatalf("rendered %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTemplatesRefuseARepeatedCapability(t *testing.T) {
	t.Parallel()
	tree := conditionTree()
	tree["templates/manifest.yml"] = &fstest.MapFile{Data: []byte(strings.Replace(string(tree["templates/manifest.yml"].Data), "when: [db]", "when: [db, db]", 1))}
	_, err := assets.Templates(tree)
	if !errors.Is(err, assets.ErrManifest) || !strings.Contains(err.Error(), `capability "db" repeats in the condition of templates/db.yml.tmpl`) {
		t.Fatalf("Templates = %v", err)
	}
	if _, err := assets.Templates(conditionTree()); err != nil {
		t.Fatalf("distinct capabilities: %v", err)
	}
}
