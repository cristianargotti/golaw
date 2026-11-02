package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Red row: a checkout under the api and db capabilities that wrote its
// OpenAPI document and no database yet runs the generators its contracts
// feed alone, so the command golaw gen --check names fixes the drift
// without a database the person has not opted into; sqlc is named not
// applicable. Near row: once the migrations exist, sqlc runs again.
func TestGenRunsOnlyTheGeneratorsTheCheckoutFeeds(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		database bool
		want     string // the generator golaw gen fails on first
	}{
		"red: no database yet":       {want: "find oapi-codegen"},
		"near: the database written": {database: true, want: "find sqlc"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ta, root := newPort(t)
			ta = missingGo(t, ta)
			writeRepoFile(t, root, "tools/go.mod", "module example.com/slice/tools\n\ngo 1.27\n")
			writeRepoFile(t, root, "sqlc.yaml", "version: \"2\"\n")
			writeRepoFile(t, root, "api/codegen/server.yaml", "package: apigen\n")
			if !tc.database {
				removeDatabase(t, root)
			}
			code := ta.run(t.Context(), []string{"gen"})
			out := ta.stdout.String() + ta.stderr.String()
			skipped := strings.Contains(out, "skipped sqlc sqlc.yaml: not applicable")
			if code != exitFailure || !strings.Contains(out, tc.want) || skipped == tc.database {
				t.Errorf("exit %d, want %d naming %q, sqlc skipped %v:\n%s", code, exitFailure, tc.want, skipped, out)
			}
		})
	}
}

// removeDatabase drops the migrations, the queries and the sqlc output, a
// checkout that never opted into its database.
func removeDatabase(t *testing.T, root string) {
	t.Helper()
	for _, dir := range []string{"db", "internal/adapters/out/postgres/sqlcgen"} {
		if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash(dir))); err != nil {
			t.Fatal(err)
		}
	}
}
