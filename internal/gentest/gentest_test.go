package gentest_test

import (
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/hometest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

func TestMain(m *testing.M) { os.Exit(runnertest.Main(m, os.Getenv)) }

func TestContractsAreTheGrowthFixture(t *testing.T) {
	t.Parallel()
	files := gentest.Contracts()
	for _, want := range []string{
		"go.mod", "golaw.yml", "internal/application/growth/ports.go", "internal/application/growth/plan.go",
		"internal/application/growth/errors.go", "db/migrations/0001_plans.sql", "db/queries/plans.sql", "api/openapi.yaml",
		"internal/adapters/out/postgres/sqlcgen/db.go", "internal/adapters/out/postgres/sqlcgen/models.go",
		"internal/adapters/out/postgres/sqlcgen/plans.sql.go", "api/gen/server.gen.go",
	} {
		if len(files[want]) == 0 {
			t.Errorf("Contracts() lacks %s", want)
		}
	}
	if !strings.HasPrefix(string(files["go.mod"]), "module example.com/slice\n") {
		t.Errorf("go.mod = %q", files["go.mod"])
	}
	for path, content := range files {
		checkFixtureFile(t, path, content)
	}
	files["go.mod"] = nil
	if again := gentest.Contracts(); len(again["go.mod"]) == 0 {
		t.Error("Contracts() answers a shared map a caller can change")
	}
}

// checkFixtureFile holds the per file assertions of the growth fixture, apart
// so the test that ranges over the files stays under the cognitive limit.
func checkFixtureFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if strings.HasSuffix(path, ".esc") || strings.Contains(string(content), "\r") {
		t.Errorf("%s keeps an escape suffix or a carriage return", path)
	}
	if strings.HasSuffix(path, ".go") {
		if _, err := parser.ParseFile(token.NewFileSet(), path, content, parser.ParseComments); err != nil {
			t.Errorf("%s does not parse: %v", path, err)
		}
	}
	if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
		var doc map[string]any
		if err := yaml.Unmarshal(content, &doc); err != nil {
			t.Errorf("%s does not decode: %v", path, err)
		}
	}
	if strings.Contains(string(content), "http://") {
		t.Errorf("%s carries a clear text URL", path)
	}
}

func TestSnapshotClassifiesLikeTheWorktree(t *testing.T) {
	t.Parallel()
	s := gentest.Snapshot(t, gentest.Contracts())
	tests := map[string]model.FileClass{
		"internal/application/growth/ports.go":                model.ClassTracked,
		"internal/adapters/out/postgres/sqlcgen/plans.sql.go": model.ClassGenerated,
		"api/gen/server.gen.go":                               model.ClassGenerated,
		"internal/application/growth/absent.go":               model.ClassUnknown,
	}
	for path, want := range tests {
		if got := s.Class(path); got != want {
			t.Errorf("Class(%s) = %v, want %v", path, got, want)
		}
	}
	if _, err := s.Read("internal/application/growth/absent.go"); err == nil {
		t.Error("Read of an absent path answered no error")
	}
	paths := make([]string, 0, len(s.Files()))
	for _, f := range s.Files() {
		paths = append(paths, f.Path)
	}
	if !slices.IsSorted(paths) || len(paths) != len(gentest.Contracts()) {
		t.Errorf("Files() = %v", paths)
	}
}

func TestModuleCommitsTheBase(t *testing.T) {
	t.Parallel()
	repo := gentest.Module(t, gentest.Contracts())
	if repo.Head() == "" {
		t.Fatal("Module left no base commit")
	}
	if got := repo.Git("ls-files", "api/openapi.yaml"); got != "api/openapi.yaml" {
		t.Errorf("ls-files = %q", got)
	}
}

func TestFakeContractsAnswerTheirFixedValues(t *testing.T) {
	t.Parallel()
	port := genkind.Port{Name: "Ledger", Package: "growth"}
	f := gentest.FakeContracts{
		PortAnswers:      map[string]genkind.Port{"internal/application/growth/ports.go#Ledger": port},
		OperationAnswers: []genkind.Operation{{Name: "GetPlan", Tag: "plans"}},
		QueryAnswers:     []genkind.Query{{Name: "ReadPlan"}},
	}
	got, err := f.Port("internal/application/growth/ports.go", "Ledger")
	if err != nil || got.Name != "Ledger" {
		t.Errorf("Port = %+v, %v", got, err)
	}
	if _, err := f.Port("internal/application/growth/ports.go", "Clock"); err == nil {
		t.Error("an unprogrammed port answered no error")
	}
	if ops, opsErr := f.Operations("api/gen/server.gen.go", "api/openapi.yaml"); opsErr != nil || len(ops) != 1 {
		t.Errorf("Operations = %+v, %v", ops, opsErr)
	}
	if qs, qsErr := f.Queries("internal/adapters/out/postgres/sqlcgen"); qsErr != nil || len(qs) != 1 {
		t.Errorf("Queries = %+v, %v", qs, qsErr)
	}
}

func TestInstallGeneratorsLaysOutTheHookGenerators(t *testing.T) {
	t.Parallel()
	dir := hometest.New(t).Tools()
	paths := gentest.InstallGenerators(t, dir, runnertest.Behavior{Stdout: "ok"})
	if len(paths) != 2 || paths["sqlc"] == "" || paths["oapi-codegen"] == "" || paths["goverter"] != "" {
		t.Fatalf("InstallGenerators = %v, want sqlc and oapi-codegen only (goverter never runs in a hook)", paths)
	}
	for name := range paths {
		if _, ok, err := toolchain.ReadReceipt(dir, name); !ok || err != nil {
			t.Errorf("no receipt for %s: %v", name, err)
		}
	}
}
