package thirdparty_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/thirdparty"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

var errUnreadable = errors.New("unreadable in the test")

// unreadable is a snapshot that refuses to read one path, as a file over
// the read bound would.
type unreadable struct {
	model.Snapshot

	path string
}

func (u unreadable) Read(p string) ([]byte, error) {
	if p == u.path {
		return nil, errUnreadable
	}
	content, err := u.Snapshot.Read(p)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", p, err)
	}
	return content, nil
}

func sqlcSnapshot(t *testing.T) model.Snapshot {
	t.Helper()
	files := gentest.Contracts()
	files["sqlc.yaml"] = []byte("version: \"2\"\n")
	return gentest.Snapshot(t, files)
}

// Red rows: a stage that cannot be written or read fails with its cause and
// never leaves the generator a partial input it would run over.
func TestStageFailsWithTheCause(t *testing.T) {
	t.Parallel()
	s := sqlcSnapshot(t)
	if err := thirdparty.Stage(unreadable{s, "db/queries/plans.sql"}, sqlc, t.TempDir()); !errors.Is(err, errUnreadable) {
		t.Errorf("an unreadable input = %v", err)
	}
	escaping := sqlc
	escaping.Outputs = []string{"../outside"}
	if err := thirdparty.Stage(s, escaping, t.TempDir()); !errors.Is(err, source.ErrEscapedRoot) {
		t.Errorf("an output outside the stage = %v", err)
	}
	blocked := t.TempDir()
	if err := os.WriteFile(filepath.Join(blocked, "db"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := thirdparty.Stage(s, sqlc, blocked); err == nil {
		t.Error("a file where an input directory goes staged nothing and passed")
	}
	taken := t.TempDir()
	if err := os.MkdirAll(filepath.Join(taken, "sqlc.yaml"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := thirdparty.Stage(s, sqlc, taken); err == nil {
		t.Error("a directory where the configuration goes staged nothing and passed")
	}
	outBlocked := t.TempDir()
	if err := os.WriteFile(filepath.Join(outBlocked, "internal"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	noInputs := sqlc
	noInputs.Config, noInputs.Inputs = "absent.yaml", nil
	if err := thirdparty.Stage(s, noInputs, outBlocked); err == nil {
		t.Error("a file where the output directory goes passed")
	}
}

func TestCompareReportsAnOutputTheGeneratorNeverWrote(t *testing.T) {
	t.Parallel()
	s := sqlcSnapshot(t)
	out := "internal/adapters/out/postgres/sqlcgen"
	got, err := thirdparty.Compare(s, sqlc, t.TempDir())
	want := []string{out + "/db.go", out + "/models.go", out + "/plans.sql.go"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Compare = %v, %v, want %v", got, err, want)
	}
	if _, err := thirdparty.Compare(unreadable{s, out + "/db.go"}, sqlc, t.TempDir()); !errors.Is(err, errUnreadable) {
		t.Errorf("an unreadable checkout output = %v", err)
	}
}

// Near row: a stage that holds the checkout's outputs byte for byte drifts
// nowhere.
func TestCompareFindsNoDriftOverEqualOutputs(t *testing.T) {
	t.Parallel()
	files := gentest.Contracts()
	s := gentest.Snapshot(t, files)
	dir := t.TempDir()
	out := "internal/adapters/out/postgres/sqlcgen"
	for _, name := range []string{"db.go", "models.go", "plans.sql.go"} {
		target := filepath.Join(dir, filepath.FromSlash(out), name)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, files[out+"/"+name], 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := thirdparty.Compare(s, sqlc, dir); err != nil || len(got) != 0 {
		t.Errorf("Compare = %v, %v, want no drift", got, err)
	}
}
