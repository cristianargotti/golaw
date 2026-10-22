package cursor_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/cursor"
	"github.com/cristianargotti/golaw/internal/model"
)

// TestCursorReadRefusesANameItCannotRead holds a beforeReadFile event whose
// file_path is absent, of another type or empty to cannot judge, never a
// read judged by a name the payload did not give.
func TestCursorReadRefusesANameItCannotRead(t *testing.T) {
	t.Parallel()
	base := curBytes(t, "proof/testdata/read/near/payload.json")
	for _, test := range []struct {
		name string
		path any
		bad  bool
	}{
		{"red number", 7, true},
		{"red empty", "", true},
		{"red null", nil, true},
		{"near source", "/project/main.go", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := curDecode(t, curPatch(t, base, map[string]any{"file_path": test.path}))
			if test.bad != errors.Is(err, model.ErrCannotJudge) || !test.bad && err != nil {
				t.Fatalf("bad = %v, error = %v", test.bad, err)
			}
		})
	}
	absent := []byte(`{"hook_event_name":"beforeReadFile","conversation_id":"conversation","workspace_roots":["/project"]}`)
	if _, err := curDecode(t, absent); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatalf("absent file_path: %v", err)
	}
}

// TestCursorReadNamesTheEntryAndTheTarget names a read twice when a link
// stands between the name and the file, as the Claude Read does: the entry
// from the root, and the file the link leads to by its absolute path when
// the repository does not hold it; a name outside the repository keeps its
// own spelling, so a credential store of the home is judged by its name.
func TestCursorReadNamesTheEntryAndTheTarget(t *testing.T) {
	t.Parallel()
	root, outside := curRealDir(t), curRealDir(t)
	store := filepath.Join(outside, ".aws", "credentials")
	if err := os.MkdirAll(filepath.Dir(store), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store, []byte("[default]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(store, filepath.Join(root, "config.txt")); err != nil {
		t.Skipf("symbolic links are not available: %v", err)
	}
	for _, test := range []struct{ name, path, real string }{
		{filepath.Join(root, "config.txt"), "config.txt", filepath.ToSlash(store)},
		{store, filepath.ToSlash(store), ""},
		{filepath.Join(root, "notes.md"), "notes.md", ""},
	} {
		read := curReadAt(t, root, test.name)
		if read.Path != test.path || read.Real != test.real {
			t.Errorf("%s: %+v", test.name, read)
		}
	}
}

func curRealDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// curReadAt decodes the read of name in a session whose only workspace root
// is root, the read's working directory (D7).
func curReadAt(t *testing.T, root, name string) model.ReadRequest {
	t.Helper()
	data := curPatch(t, curBytes(t, "proof/testdata/read/near/payload.json"), map[string]any{
		"workspace_roots": []string{filepath.ToSlash(root)},
		"file_path":       filepath.ToSlash(name),
	})
	parsed, err := cursor.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Event.Target != filepath.ToSlash(name) || parsed.Event.Cwd != filepath.ToSlash(root) {
		t.Fatalf("header = %+v", parsed.Event)
	}
	request, err := parsed.Decode(hook.Base{Root: root}, nil)
	if err != nil || request.Event.Read == nil {
		t.Fatalf("read = %+v, %v", request, err)
	}
	return *request.Event.Read
}
