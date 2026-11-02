package apply_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/apply"
	"github.com/cristianargotti/golaw/internal/source"
)

func put(t *testing.T, root, rel, content string, mode fs.FileMode) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// leftovers are the temporary files a write would leave behind.
func leftovers(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && strings.HasPrefix(d.Name(), ".golaw-gen-") {
			out = append(out, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestFilesWritesNewFilesAndReplacesKeepingTheMode(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	put(t, root, "cmd/api/zz_generated_wiring.go", "old\n", 0o644)
	writes := []gen.Write{
		{Path: "internal/gensupport/zz_generated_hole.go", Content: []byte("hole\n"), Mode: gen.WriteRegenerable},
		{Path: "cmd/api/zz_generated_wiring.go", Content: []byte("new\n"), Mode: gen.WriteRegenerable, Base: source.ContentHash([]byte("old\n"))},
		{Path: ".golaw/gen.yml", Content: []byte("golaw-gen: 1\n"), Mode: gen.WriteManifest},
	}
	report, err := apply.Files(root, writes)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".golaw/gen.yml", "cmd/api/zz_generated_wiring.go", "internal/gensupport/zz_generated_hole.go"}
	if !reflect.DeepEqual(report.Written, want) || len(report.Skipped) != 0 {
		t.Fatalf("report = %+v", report)
	}
	if read(t, root, "cmd/api/zz_generated_wiring.go") != "new\n" || read(t, root, "internal/gensupport/zz_generated_hole.go") != "hole\n" {
		t.Error("a write did not land")
	}
	if runtime.GOOS != "windows" {
		modes := map[string]fs.FileMode{"cmd/api/zz_generated_wiring.go": 0o644, "internal/gensupport/zz_generated_hole.go": 0o600, "internal/gensupport": 0o700}
		for rel, mode := range modes {
			info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil || info.Mode().Perm() != mode {
				t.Errorf("%s mode = %v, %v, want %v", rel, info.Mode().Perm(), err, mode)
			}
		}
	}
	if left := leftovers(t, root); len(left) != 0 {
		t.Errorf("temporary files left: %v", left)
	}
}

// Red rows: a file that changed since the plan read it and an owned file
// that exists are skipped with their cause, never overwritten.
func TestFilesSkipsWhatItMustNotOverwrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	put(t, root, "internal/adapters/out/postgres/plans.go", "edited by the agent\n", 0o600)
	put(t, root, "internal/adapters/out/postgres/plans_test.go", "owned\n", 0o600)
	put(t, root, "internal/adapters/out/postgres/zz_generated_plans.go", "appeared\n", 0o600)
	writes := []gen.Write{
		{Path: "internal/adapters/out/postgres/plans.go", Content: []byte("stub\n"), Mode: gen.WriteAppend, Base: source.ContentHash([]byte("read by the hook\n"))},
		{Path: "internal/adapters/out/postgres/plans_test.go", Content: []byte("again\n"), Mode: gen.WriteOwned},
		{Path: "internal/adapters/out/postgres/zz_generated_plans.go", Content: []byte("rendered\n"), Mode: gen.WriteRegenerable},
	}
	report, err := apply.Files(root, writes)
	want := []apply.Skip{
		{Path: "internal/adapters/out/postgres/plans.go", Cause: apply.CauseChanged},
		{Path: "internal/adapters/out/postgres/plans_test.go", Cause: apply.CauseOwned},
		{Path: "internal/adapters/out/postgres/zz_generated_plans.go", Cause: apply.CauseChanged},
	}
	if err != nil || len(report.Written) != 0 || !reflect.DeepEqual(report.Skipped, want) {
		t.Fatalf("report = %+v, %v", report, err)
	}
	if read(t, root, "internal/adapters/out/postgres/plans.go") != "edited by the agent\n" {
		t.Error("a changed file was overwritten")
	}
}

func TestFilesRefusesAPathOutsideTheRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, rel := range []string{"../outside.go", "/etc/x.go"} {
		_, err := apply.Files(root, []gen.Write{{Path: rel, Content: []byte("x\n"), Mode: gen.WriteRegenerable}})
		if !errors.Is(err, apply.ErrOutside) {
			t.Errorf("Files(%s) = %v, want ErrOutside", rel, err)
		}
	}
	if runtime.GOOS == "windows" {
		t.Skip("creating a symbolic link needs a privilege the Windows runner lacks")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := apply.Files(root, []gen.Write{{Path: "link/x.go", Content: []byte("x\n"), Mode: gen.WriteRegenerable}}); err == nil {
		t.Error("a write through a link out of the root answered no error")
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Errorf("the write escaped the root: %v, %v", entries, err)
	}
}

func TestFilesStopsAtAFailureAndReportsWhatItWrote(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	put(t, root, "b", "a file where a directory is needed\n", 0o600)
	writes := []gen.Write{
		{Path: "a.go", Content: []byte("a\n"), Mode: gen.WriteRegenerable},
		{Path: "b/c.go", Content: []byte("c\n"), Mode: gen.WriteRegenerable},
		{Path: "d.go", Content: []byte("d\n"), Mode: gen.WriteRegenerable},
	}
	report, err := apply.Files(root, writes)
	if err == nil || !reflect.DeepEqual(report.Written, []string{"a.go"}) {
		t.Fatalf("report = %+v, %v", report, err)
	}
	if _, err := os.Stat(filepath.Join(root, "d.go")); !os.IsNotExist(err) {
		t.Errorf("a write after the failure landed: %v", err)
	}
	if left := leftovers(t, root); len(left) != 0 {
		t.Errorf("temporary files left: %v", left)
	}
}
