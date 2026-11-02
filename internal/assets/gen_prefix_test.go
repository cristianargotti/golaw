package assets_test

import (
	"errors"
	"testing"
	"testing/fstest"

	"github.com/cristianargotti/golaw/internal/assets"
)

func bundle(files ...string) fstest.MapFS {
	tree := fstest.MapFS{
		"agents/manifest.json": {Data: []byte(`{"files": ["agents/manifest.json", "templates/a.tmpl"]}`)},
		"templates/a.tmpl":     {Data: []byte("a\n")},
	}
	for _, name := range files {
		tree[name] = &fstest.MapFile{Data: []byte("x\n")}
	}
	return tree
}

func TestTheGeneratorTreeIsOutsideTheAgentsManifest(t *testing.T) {
	t.Parallel()
	if assets.GenPrefix != "templates/gen/" {
		t.Errorf("GenPrefix = %q", assets.GenPrefix)
	}
	tests := map[string]struct {
		source fstest.MapFS
		ok     bool
	}{
		"red: a kind template unlisted":       {source: bundle("templates/gen/port/1/zz_generated_fake.go.tmpl"), ok: true},
		"red: a nested support template":      {source: bundle("templates/gen/support/1/sqltest/zz_generated_driver.go.tmpl"), ok: true},
		"near: another template unlisted":     {source: bundle("templates/other.tmpl")},
		"near: a sibling named like the tree": {source: bundle("templates/generated.tmpl")},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := assets.Load(tc.source)
			if tc.ok && err != nil {
				t.Errorf("Load = %v, want nil", err)
			}
			if !tc.ok && !errors.Is(err, assets.ErrManifest) {
				t.Errorf("Load = %v, want ErrManifest", err)
			}
		})
	}
}

func TestAListedGeneratorTemplateIsRefused(t *testing.T) {
	t.Parallel()
	tree := bundle("templates/gen/port/1/x.go.tmpl")
	tree["agents/manifest.json"] = &fstest.MapFile{Data: []byte(`{"files": ["agents/manifest.json", "templates/a.tmpl", "templates/gen/port/1/x.go.tmpl"]}`)}
	if _, err := assets.Load(tree); !errors.Is(err, assets.ErrManifest) {
		t.Errorf("Load = %v, want ErrManifest: the kinds own templates/gen", err)
	}
}
