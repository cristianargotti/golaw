package assets_test

import (
	"testing"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
)

func assetContext() assets.Context {
	return assets.Context{Values: templateValues(), CIBackend: "none"}
}

func renderedAsset(t *testing.T, path string, c assets.Context) assets.Rendered {
	t.Helper()
	files, err := assets.RenderAllContext(golaw.FS, c)
	if err != nil {
		t.Fatal(err)
	}
	for i := range files {
		if files[i].Path == path {
			return files[i]
		}
	}
	t.Fatalf("missing asset: %s", path)
	return assets.Rendered{}
}

func assertAsset(t *testing.T, path, want string, ownership assets.Ownership) {
	t.Helper()
	got := renderedAsset(t, path, assetContext())
	if string(got.Data) != want || got.Template.Ownership != ownership {
		t.Fatalf("asset %s = %q, ownership %q", path, got.Data, got.Template.Ownership)
	}
	if len(got.Template.When) != 0 {
		t.Fatalf("unexpected condition on %s", path)
	}
}

func yamlMap(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := parent[key].(map[string]any)
	if !ok {
		t.Fatalf("%s is not a mapping", key)
	}
	return value
}
