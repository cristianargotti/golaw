package policy_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestRecordRoundTrip(t *testing.T) {
	t.Parallel()
	state := filepath.Join(t.TempDir(), "state")
	root := filepath.Join(t.TempDir(), "repo")
	if _, found, err := policy.ReadRecord(state, root); err != nil || found {
		t.Errorf("no record yet: %v, %v", found, err)
	}
	writeRecord(t, state, record(root))
	got, found, err := policy.ReadRecord(state, root)
	if err != nil || !found {
		t.Fatalf("ReadRecord = %v, %v", found, err)
	}
	want := record(root)
	want.Schema = policy.Schema
	if got != want {
		t.Errorf("ReadRecord = %+v, want %+v", got, want)
	}
	path := policy.RecordPath(state, root)
	if !strings.HasPrefix(path, filepath.Join(state, "enrollments")) || !strings.HasSuffix(path, ".json") || policy.RecordPath(state, root+"2") == path {
		t.Errorf("RecordPath = %s", path)
	}
	if policy.Identity(root) != filepath.ToSlash(root) {
		t.Errorf("Identity = %s", policy.Identity(root))
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Errorf("no temporary file is left behind: %v, %v", entries, err)
	}
	if err := policy.WriteRecord(state, policy.Record{Root: root}); !errors.Is(err, model.ErrUsage) {
		t.Errorf("an incomplete record is refused: %v", err)
	}
}

func TestRecordDamaged(t *testing.T) {
	t.Parallel()
	state := filepath.Join(t.TempDir(), "state")
	root := filepath.Join(t.TempDir(), "repo")
	for name, content := range map[string]string{"not json": "{", "incomplete": `{"golaw": 1, "root": ""}`, "schema": `{"golaw": 2, "root": "r", "policy_digest": "d"}`} {
		write(t, filepath.Dir(policy.RecordPath(state, root)), filepath.Base(policy.RecordPath(state, root)), content)
		if _, _, err := policy.ReadRecord(state, root); !errors.Is(err, model.ErrEnrollment) {
			t.Errorf("%s: %v, want ErrEnrollment", name, err)
		}
	}
	if runtime.GOOS != "windows" && os.Getuid() != 0 {
		if err := os.Chmod(policy.RecordPath(state, root), 0o000); err != nil {
			t.Fatal(err)
		}
		if _, _, err := policy.ReadRecord(state, root); !errors.Is(err, model.ErrEnrollment) {
			t.Errorf("unreadable: %v", err)
		}
	}
	if err := policy.WriteRecord(filepath.Join(policy.RecordPath(state, root), "below"), record(root)); err == nil {
		t.Error("a state directory under a file cannot be created")
	}
}

func TestFindRecord(t *testing.T) {
	t.Parallel()
	state := filepath.Join(t.TempDir(), "state")
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "a/b/c.txt", "x\n")
	_, found, err := policy.FindRecord(state, filepath.Join(root, "a", "b"))
	if err != nil || found {
		t.Errorf("nothing enrolled: %v, %v", found, err)
	}
	writeRecord(t, state, record(root))
	got, found, err := policy.FindRecord(state, filepath.Join(root, "a", "b"))
	if err != nil || !found || got.Root != root {
		t.Errorf("FindRecord from a subdirectory = %+v, %v, %v", got, found, err)
	}
	_, _, err = policy.FindRecord(state, filepath.Join(root, "absent"))
	if !errors.Is(err, model.ErrUsage) {
		t.Errorf("an absent directory: %v", err)
	}
}
