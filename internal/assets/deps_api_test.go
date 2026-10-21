package assets_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
)

func TestAPITemplates(t *testing.T) {
	t.Parallel()
	paths := []string{
		"redocly.yaml", "api/codegen/types.yaml",
		"api/codegen/server.yaml", "api/codegen/spec.yaml",
	}
	for _, capabilities := range [][]string{{"go"}, {"go", "api"}} {
		context := assetContext()
		context.Capabilities = capabilities
		rendered, err := assets.RenderAllContext(golaw.FS, context)
		if err != nil {
			t.Fatal(err)
		}
		requireAPIInventory(t, rendered, paths, capabilities)
	}
	context := assetContext()
	context.Capabilities = []string{"go", "api"}
	for _, tc := range []struct{ path, feature, output string }{
		{"api/codegen/types.yaml", "models: true", "api/generated/types.gen.go"},
		{"api/codegen/server.yaml", "strict-server: true", "api/generated/server.gen.go"},
		{"api/codegen/spec.yaml", "embedded-spec: true", "api/generated/spec.gen.go"},
	} {
		body := string(renderedAsset(t, tc.path, context).Data)
		if !strings.Contains(body, tc.feature) || !strings.Contains(body, tc.output) {
			t.Fatalf("asset %s = %q", tc.path, body)
		}
	}
}

func requireAPIInventory(t *testing.T, rendered []assets.Rendered, paths, capabilities []string) {
	t.Helper()
	for _, path := range paths {
		index := slices.IndexFunc(rendered, func(file assets.Rendered) bool {
			return file.Path == path
		})
		if !slices.Contains(capabilities, "api") {
			if index >= 0 {
				t.Fatalf("unexpected API asset %s", path)
			}
			continue
		}
		if index < 0 {
			t.Fatalf("missing API asset %s", path)
		}
		file := rendered[index]
		if file.Template.Ownership != assets.OwnedByKit ||
			!slices.Equal(file.Template.When, []string{"api"}) {
			t.Fatalf("metadata = %+v", file.Template)
		}
	}
}
