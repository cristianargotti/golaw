package hook

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

// TestRootedShellSpellsTheCwdAsTheRootIsSpelled: discovery names the root
// by its resolved path, and a host whose working directory reaches it through
// a link (the temporary directory of macOS) must still place the literal
// targets of a shell command inside the repository.
func TestRootedShellSpellsTheCwdAsTheRootIsSpelled(t *testing.T) {
	t.Parallel()
	resolved, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(resolved, "repo")
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o750); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(resolved, "link")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("no symbolic link on this system: %v", err)
	}
	shell := &model.ShellCommand{Command: "echo x > a.go", Cwd: filepath.Join(link, "internal"), Tool: "Bash"}
	got := rootedShell(shell, root)
	if got.Root != root || got.Cwd != filepath.Join(root, "internal") {
		t.Fatalf("rooted = %+v, want the root %s and the cwd under it", got, root)
	}
	if shell.Root != "" || shell.Cwd != filepath.Join(link, "internal") {
		t.Fatalf("the request's shell changed: %+v", shell)
	}
	near := rootedShell(&model.ShellCommand{Command: "echo x > a.go", Tool: "Bash"}, root)
	if near.Cwd != "" || near.Root != root {
		t.Fatalf("a payload without a cwd = %+v", near)
	}
	if rootedShell(nil, root) != nil {
		t.Fatal("a nil shell was rooted")
	}
}
