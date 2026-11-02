package thirdparty_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/thirdparty"
	"github.com/cristianargotti/golaw/internal/gentest"
)

func readStaged(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestStageCopiesTheConfigurationAndTheInputsOnly(t *testing.T) {
	t.Parallel()
	files := gentest.Contracts()
	files["sqlc.yaml"] = []byte("version: \"2\"\n")
	s := gentest.Snapshot(t, files)
	dir := t.TempDir()
	if err := thirdparty.Stage(s, sqlc, dir); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"sqlc.yaml", "db/queries/plans.sql", "db/migrations/0001_plans.sql"} {
		if readStaged(t, dir, rel) != string(files[rel]) {
			t.Errorf("%s was not staged as it is", rel)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "internal", "application")); !os.IsNotExist(err) {
		t.Errorf("Stage copied a file the generator does not read: %v", err)
	}
	if info, err := os.Stat(filepath.Join(dir, "internal", "adapters", "out", "postgres", "sqlcgen")); err != nil || !info.IsDir() {
		t.Errorf("the output directory was not prepared: %v", err)
	}
}

func TestCompareNamesEveryOutputThatDiffers(t *testing.T) {
	t.Parallel()
	files := gentest.Contracts()
	s := gentest.Snapshot(t, files)
	dir := t.TempDir()
	out := "internal/adapters/out/postgres/sqlcgen"
	if err := os.MkdirAll(filepath.Join(dir, filepath.FromSlash(out)), 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(rel, content string) {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(out+"/db.go", string(files[out+"/db.go"]))              // equal: no drift
	write(out+"/models.go", string(files[out+"/models.go"])+"//") // differs
	write(out+"/querier.go", "package sqlcgen\n")                 // the generator writes it, the checkout lacks it
	got, err := thirdparty.Compare(s, sqlc, dir)
	want := []string{out + "/models.go", out + "/plans.sql.go", out + "/querier.go"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Compare = %v, %v, want %v (plans.sql.go exists only in the checkout)", got, err, want)
	}
}
