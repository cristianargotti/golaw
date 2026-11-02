package bootstrap_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
)

const (
	recordPath = "docs/adr/0001-inherited-baseline.md"
	recordHead = "---\nrecord: 1\ntitle: inherited dependency baseline\nstatus: inherited\n"
	recordBody = "---\n\n# Dependências herdadas\n"
	kitTools   = "module example.test/app/tools\n\ngo 1.27\n\nrequire (\n\tgithub.com/evilmartians/lefthook/v2 v2.1.12\n\tgolang.org/x/vuln v1.7.0\n)\n"
	kitLines   = "  - github.com/quasilyte/go-ruleguard/dsl\n  - github.com/evilmartians/lefthook/v2\n  - golang.org/x/vuln\n"
)

// TestUpdateProposesTheKitModulesTheRecordsLack holds the first update of
// an adoption whose baseline record predates the measured tools module: the
// modules the kit's go.mod lines and tools/go.mod require, which no
// decision records, are proposed into the baseline record's modules, in the
// shapes an earlier init wrote, while a module a decision already records
// and a repository that never adopted the templates get nothing.
func TestUpdateProposesTheKitModulesTheRecordsLack(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, record, other, want string
		adopted                   bool
	}{
		{"red empty flow list", recordHead + "modules:\n  []\n" + recordBody, "", recordHead + "modules:\n" + kitLines + recordBody, true},
		{"red inline empty list", recordHead + "modules: []\n" + recordBody, "", recordHead + "modules:\n" + kitLines + recordBody, true},
		{"red block list", recordHead + "modules:\n  - example.test/lib\n" + recordBody, "", recordHead + "modules:\n  - example.test/lib\n" + kitLines + recordBody, true},
		{"red missing only", recordHead + "modules:\n  - golang.org/x/vuln\n" + recordBody, "", recordHead + "modules:\n  - golang.org/x/vuln\n" +
			"  - github.com/quasilyte/go-ruleguard/dsl\n  - github.com/evilmartians/lefthook/v2\n" + recordBody, true},
		{"near recorded elsewhere", recordHead + "modules:\n  []\n" + recordBody, "---\nmodules:\n" + kitLines + "---\n", "", true},
		{"near already recorded", recordHead + "modules:\n" + kitLines + recordBody, "", "", true},
		{"near unknown shape", recordHead + "modules: [example.test/lib]\n" + recordBody, "", "", true},
		{"near never adopted", recordHead + "modules:\n  []\n" + recordBody, "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := proposedRecord(t, tc.record, tc.other, tc.adopted)
			if got != tc.want {
				t.Fatalf("record proposal = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestUpdateProposesTheRuntimesOfTheGeneratedCode holds the update of an
// adoption whose tools module pins oapi-codegen: the code golaw gen writes
// with it imports kin-openapi and the oapi-codegen runtime, so the record
// proposal names them beside the generator, or golaw scan denies the
// modules of code the kit generated.
func TestUpdateProposesTheRuntimesOfTheGeneratedCode(t *testing.T) {
	t.Parallel()
	tools := "module example.test/app/tools\n\ngo 1.27\n\nrequire github.com/oapi-codegen/oapi-codegen/v2 v2.8.0\n"
	for name, tc := range map[string]struct{ tools, want string }{
		"red: the oapi generator": {tools, recordHead + "modules:\n  - github.com/quasilyte/go-ruleguard/dsl\n" +
			"  - github.com/oapi-codegen/oapi-codegen/v2\n  - github.com/getkin/kin-openapi\n  - github.com/oapi-codegen/runtime\n" + recordBody},
		"near: no generator": {kitTools, recordHead + "modules:\n" + kitLines + recordBody},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := proposedRecordOver(t, recordHead+"modules:\n  []\n"+recordBody, "", tc.tools, true); got != tc.want {
				t.Fatalf("record proposal = %q, want %q", got, tc.want)
			}
		})
	}
}

// proposedRecord plans the update of a repository holding the record, and
// another decision when given, and answers the record proposal, empty when
// the plan proposes none.
func proposedRecord(t *testing.T, record, other string, adopted bool) string {
	t.Helper()
	return proposedRecordOver(t, record, other, kitTools, adopted)
}

// proposedRecordOver is proposedRecord over another tools module.
func proposedRecordOver(t *testing.T, record, other, tools string, adopted bool) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "adr"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeCurrent(t, filepath.Join(root, filepath.FromSlash(recordPath)), record)
	writeCurrent(t, filepath.Join(root, "docs", "adr", "0002-tools.md"), other)
	writeCurrent(t, filepath.Join(root, "go.mod"), "module example.test/app\n")
	var recorded []policy.ManagedFile
	if adopted {
		recorded = []policy.ManagedFile{{Path: "tools/go.mod", SHA256: install.Digest([]byte(tools))}}
	}
	plan, err := bootstrap.PlanUpdate(root, []assets.Rendered{
		templateFile("go.mod", "require github.com/quasilyte/go-ruleguard/dsl v0.3.23\n", assets.OwnedMerged),
		templateFile("tools/go.mod", tools, assets.OwnedByKit),
		templateFile(recordPath, recordHead+"modules:\n  - fresh\n"+recordBody, assets.OwnedByDeveloper),
	}, recorded)
	if err != nil {
		t.Fatal(err)
	}
	if index := slices.IndexFunc(plan.Proposed, func(c install.Change) bool { return c.Path == recordPath }); index >= 0 {
		return string(plan.Proposed[index].After)
	}
	return ""
}
