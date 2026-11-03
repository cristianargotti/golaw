package newcode_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/newcode"
	"github.com/cristianargotti/golaw/internal/runner"
)

// The worktree lines add to the committed ones the staged, the unstaged
// and the untracked lines, which NewLines leaves out.
func TestWorktreeLinesReadTheUncommittedChange(t *testing.T) {
	t.Parallel()
	repo := history(t)
	repo.Stage(map[string]string{"staged.go": "package m\n\nvar S = 1\n"})
	repo.Write("a.go", "package m\n\nfunc A() int {\n\treturn 2\n}\n\n// B is new.\nvar B = 3\n\n// C is unstaged.\nvar C = 4\n")
	repo.Write("fresh.go", "package m\n\nvar F = 5")
	repo.Write("notes.txt", "third\n")
	got, err := newcode.WorktreeLines(gitContext(t), repo.Root, "base")
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string][]int{
		"a.go": {4, 6, 7, 8, 9, 10, 11}, "staged.go": {1, 2, 3}, "fresh.go": {1, 2, 3}, "pkg/ação x.go": {1, 2, 3},
	} {
		if !reflect.DeepEqual(got[path], want) {
			t.Errorf("%s = %v, want %v", path, got[path], want)
		}
	}
	if _, ok := got["notes.txt"]; ok {
		t.Errorf("a file that is not Go has new lines: %v", got)
	}
	committed, err := newcode.NewLines(gitContext(t), repo.Root, "base")
	if err != nil || committed["fresh.go"] != nil || committed["staged.go"] != nil {
		t.Errorf("NewLines = %v, %v, want the commits alone", committed, err)
	}
}

func TestWorktreeLinesRefuseABaseThatNamesNoCommit(t *testing.T) {
	t.Parallel()
	repo := history(t)
	if _, err := newcode.WorktreeLines(gitContext(t), repo.Root, "nowhere"); !errors.Is(err, newcode.ErrBase) {
		t.Fatalf("err = %v, want ErrBase", err)
	}
}

// A near miss: a worktree equal to its base adds no line, an empty
// untracked file included.
func TestWorktreeLinesOfAnUnchangedWorktreeAreEmpty(t *testing.T) {
	t.Parallel()
	repo := history(t)
	repo.Write("empty.go", "")
	got, err := newcode.WorktreeLines(gitContext(t), repo.Root, "HEAD")
	if err != nil || len(got) != 0 {
		t.Fatalf("lines = %v, %v, want none", got, err)
	}
}

// A red row of the bound: a worktree diff the runner would cut is refused,
// because a cut diff names fewer new lines.
func TestWorktreeLinesRefuseADiffOverTheBound(t *testing.T) {
	t.Parallel()
	repo := history(t)
	repo.Write("a.go", "package m\n\n"+strings.Repeat("// a long comment line of the generated kind\n", runner.MaxOutputBytes/40))
	if _, err := newcode.WorktreeLines(gitContext(t), repo.Root, "base"); !errors.Is(err, newcode.ErrDiff) {
		t.Fatalf("err = %v, want ErrDiff", err)
	}
}

// An untracked link to nothing adds no line and fails nothing.
func TestWorktreeLinesSkipAnUntrackedFileTheyCannotRead(t *testing.T) {
	t.Parallel()
	repo := history(t)
	if err := os.Symlink(filepath.Join(repo.Root, "absent"), filepath.Join(repo.Root, "dangling.go")); err != nil {
		t.Skipf("this host makes no symbolic link: %v", err)
	}
	got, err := newcode.WorktreeLines(gitContext(t), repo.Root, "HEAD")
	if err != nil || len(got) != 0 {
		t.Fatalf("lines = %v, %v, want none", got, err)
	}
}
