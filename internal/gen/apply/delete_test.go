package apply_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/apply"
	"github.com/cristianargotti/golaw/internal/source"
)

func TestFilesDeletesOnlyTheBytesTheRunRead(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	put(t, root, "internal/adapters/out/postgres/zz_generated_plans.go", "rendered\n", 0o600)
	put(t, root, "internal/adapters/out/postgres/zz_generated_replica.go", "edited\n", 0o600)
	rendered := source.ContentHash([]byte("rendered\n"))
	writes := []gen.Write{
		{Path: "internal/adapters/out/postgres/zz_generated_plans.go", Mode: gen.WriteDelete, Base: rendered},
		{Path: "internal/adapters/out/postgres/zz_generated_replica.go", Mode: gen.WriteDelete, Base: rendered},
		{Path: "internal/adapters/out/postgres/zz_generated_gone.go", Mode: gen.WriteDelete, Base: rendered},
	}
	report, err := apply.Files(root, writes)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(report.Written, []string{"internal/adapters/out/postgres/zz_generated_plans.go"}) {
		t.Errorf("written = %v, want the file whose bytes the run read", report.Written)
	}
	if want := []apply.Skip{{Path: "internal/adapters/out/postgres/zz_generated_replica.go", Cause: apply.CauseChanged}}; !reflect.DeepEqual(report.Skipped, want) {
		t.Errorf("skipped = %+v, want the edited file kept", report.Skipped)
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "adapters", "out", "postgres", "zz_generated_plans.go")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the deleted file remains: %v", err)
	}
	if got := read(t, root, "internal/adapters/out/postgres/zz_generated_replica.go"); got != "edited\n" {
		t.Errorf("the edited file = %q", got)
	}
}
