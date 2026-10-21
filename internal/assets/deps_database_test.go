package assets_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
)

func TestDatabaseTemplates(t *testing.T) {
	t.Parallel()
	for _, capabilities := range [][]string{{"go"}, {"go", "db"}} {
		t.Run(strings.Join(capabilities, ","), func(t *testing.T) {
			t.Parallel()
			context := assetContext()
			context.Capabilities = capabilities
			rendered, err := assets.RenderAllContext(golaw.FS, context)
			if err != nil {
				t.Fatal(err)
			}
			requireDatabaseInventory(t, rendered, capabilities)
		})
	}
	context := assetContext()
	context.Capabilities = []string{"go", "db"}
	main := string(renderedAsset(t, "sqlc.yaml", context).Data)
	vet := string(renderedAsset(t, "sqlc.vet.yaml", context).Data)
	if !strings.Contains(main, "many-needs-bound") ||
		!strings.Contains(main, "internal/adapters/outbound/postgres/sqlcgen") ||
		strings.Contains(main, "sqlc/db-prepare") ||
		!strings.Contains(vet, "sqlc/db-prepare") {
		t.Fatalf("database configurations = %q and %q", main, vet)
	}
}

func requireDatabaseInventory(t *testing.T, rendered []assets.Rendered, capabilities []string) {
	t.Helper()
	for _, path := range []string{".squawk.toml", "sqlc.yaml", "sqlc.vet.yaml"} {
		index := slices.IndexFunc(rendered, func(file assets.Rendered) bool {
			return file.Path == path
		})
		if !slices.Contains(capabilities, "db") {
			if index >= 0 {
				t.Fatalf("unexpected database asset %s", path)
			}
			continue
		}
		if index < 0 {
			t.Fatalf("missing database asset %s", path)
		}
		file := rendered[index]
		if file.Template.Ownership != assets.OwnedByKit ||
			!slices.Equal(file.Template.When, []string{"db"}) {
			t.Fatalf("metadata = %+v", file.Template)
		}
	}
}
