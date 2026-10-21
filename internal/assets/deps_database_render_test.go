package assets_test

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestDatabaseTemplateMessagesFollowLanguage(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ language, message string }{
		{"pt-BR", "consultas com vários resultados precisam de limite"},
		{"en", "queries returning many rows need a bound"},
	} {
		t.Run(tc.language, func(t *testing.T) {
			t.Parallel()
			context := assetContext()
			context.Capabilities = []string{"go", "db"}
			context.Language = tc.language
			file := renderedAsset(t, "sqlc.yaml", context)
			var config map[string]any
			if err := yaml.Unmarshal(file.Data, &config); err != nil {
				t.Fatal(err)
			}
			want := []any{map[string]any{
				"name": "many-needs-bound", "message": tc.message,
				"rule": `query.cmd == "many" && !query.sql.matches("(?is).*\\b(limit|fetch)\\b.*")`,
			}}
			if !reflect.DeepEqual(config["rules"], want) {
				t.Fatalf("rules = %+v, want %+v", config["rules"], want)
			}
		})
	}
}

func TestDatabasePreparationRemainsSeparate(t *testing.T) {
	t.Parallel()
	context := assetContext()
	context.Capabilities = []string{"go", "db"}
	for _, tc := range []struct {
		path string
		want map[string]any
	}{
		{
			"sqlc.yaml", map[string]any{
				"engine": "postgresql", "schema": "db/migrations", "queries": "db/queries",
				"gen": map[string]any{"go": map[string]any{
					"package": "sqlcgen", "out": "internal/adapters/outbound/postgres/sqlcgen",
					"sql_package": "database/sql",
				}},
				"rules": []any{"many-needs-bound"},
			},
		},
		{
			"sqlc.vet.yaml", map[string]any{
				"engine": "postgresql", "schema": "db/migrations", "queries": "db/queries",
				"database": map[string]any{"uri": "${DATABASE_URL}"},
				"rules":    []any{"sqlc/db-prepare"},
			},
		},
	} {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			var config map[string]any
			if err := yaml.Unmarshal(renderedAsset(t, tc.path, context).Data, &config); err != nil {
				t.Fatal(err)
			}
			if config["version"] != "2" || !reflect.DeepEqual(config["sql"], []any{tc.want}) {
				t.Fatalf("database configuration = %+v", config)
			}
		})
	}
}

func TestDatabaseSquawkTemplatePassesPolicy(t *testing.T) {
	t.Parallel()
	context := assetContext()
	context.Capabilities = []string{"go", "db"}
	file := renderedAsset(t, ".squawk.toml", context)
	tree := proof.NewTree(map[string][]byte{file.Path: file.Data})
	result, err := policyfiles.SquawkConfig().Run(t.Context(), model.Input{
		Mode: model.ModeScan, Policy: proof.Policy(), Snapshot: tree, Targets: tree.Files(),
	})
	if err != nil || result.Status.Kind != model.StatusMeasured || len(result.Findings) != 0 {
		t.Fatalf("rendered squawk configuration = %+v, error %v", result, err)
	}
}
