package shell_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/shell"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// worktree is a snapshot that classes the listed paths as untracked, the
// way the worktree snapshot of source does for a file git would add.
type worktree struct {
	files     map[string]string
	untracked []string
}

func (w worktree) Files() []model.File {
	out := make([]model.File, 0, len(w.files))
	for _, path := range slices.Sorted(func(yield func(string) bool) {
		for path := range w.files {
			if !yield(path) {
				return
			}
		}
	}) {
		out = append(out, model.File{Path: path, Class: w.Class(path), Size: int64(len(w.files[path]))})
	}
	return out
}

func (w worktree) Read(path string) ([]byte, error) {
	content, ok := w.files[path]
	if !ok {
		return nil, model.ErrNotInSnapshot
	}
	switch content {
	case "unreadable":
		return nil, model.ErrUnreadable
	case "too large":
		return nil, model.ErrTooLarge
	}
	return []byte(content), nil
}

func (w worktree) Class(path string) model.FileClass {
	if _, ok := w.files[path]; !ok {
		return model.ClassUnknown
	}
	if slices.Contains(w.untracked, path) {
		return model.ClassUntracked
	}
	return model.ClassTracked
}

// adds builds a git add line naming n files that do not exist in the tree.
func adds(n int) string {
	names := make([]string, 0, n)
	for i := range n {
		names = append(names, fmt.Sprintf("f%02d.go", i+1))
	}
	return "git add " + strings.Join(names, " ")
}

func TestShapeCountsNamedFiles(t *testing.T) {
	t.Parallel()
	denies(t, shell.Shape(), "commit-shape", map[string]bool{
		adds(41) + ` && git commit -m "chore: dump"`: true,
		adds(40) + ` && git commit -m "chore: dump"`: false,
		adds(41) + ` && git commit -m "feat: dump"`:  false,
		adds(41): false,
		adds(41) + ` && git commit --amend -m "chore: dump"`: false,
		adds(41) + ` && git commit -m "dump"`:                true,
		"git add f01.go ./f01.go && git commit -m 'fix: x'":  false,
		`git commit -m "refactor: x"`:                        false,
	})
	res := judge(t, shell.Shape(), adds(41)+` && git commit -m "dump"`)
	if !strings.Contains(res.Findings[0].Message, "a typeless commit stages 41 files and 0 new lines") {
		t.Errorf("message = %q", res.Findings[0].Message)
	}
}

func TestShapeCountsNewLines(t *testing.T) {
	t.Parallel()
	big := strings.Repeat("x\n", 401)
	half := strings.Repeat("x\n", 201)
	tree := worktree{
		files:     map[string]string{"internal/big.go": big, "internal/a.go": half, "internal/b.go": half, "docs/c.md": half, "old.go": big},
		untracked: []string{"internal/big.go", "internal/a.go", "internal/b.go", "docs/c.md"},
	}
	cases := map[string]bool{
		`git add internal/big.go && git commit -m "refactor: move"`: true,
		`git add old.go && git commit -m "refactor: move"`:          false,
		`git add . && git commit -m "refactor: move"`:               true,
		`git add -A && git commit -m "refactor: move"`:              true,
		`git add internal/ && git commit -m "refactor: move"`:       true,
		`git add ./internal/a.go && git commit -m "refactor: move"`: false,
		`git add -u && git commit -m "refactor: move"`:              false,
		`git add docs && git commit -m "refactor: move"`:            false,
		`git add internal/big.go && git commit -m "feat: move"`:     false,
	}
	for command, want := range cases {
		res, err := shell.Shape().Run(t.Context(), input(command, proof.Policy(), tree))
		if err != nil || res.Status.Kind != model.StatusMeasured {
			t.Fatalf("%q: %+v, %v", command, res, err)
		}
		if got := len(res.Findings) == 1; got != want {
			t.Errorf("%q: finding %v, want %v: %+v", command, got, want, res.Findings)
		}
	}
	res, err := shell.Shape().Run(t.Context(), input(`git add . && git commit -m "refactor: move"`, proof.Policy(), tree))
	if err != nil || !strings.Contains(res.Findings[0].Message, "stages 4 files and 1004 new lines") {
		t.Errorf("message = %q", res.Findings[0].Message)
	}
}

func TestShapeCountsAnUnterminatedLastLine(t *testing.T) {
	t.Parallel()
	tree := worktree{files: map[string]string{"big.go": strings.Repeat("x\n", 401), "tail.go": "a\nb"}, untracked: []string{"big.go", "tail.go"}}
	res, err := shell.Shape().Run(t.Context(), input(`git add big.go tail.go && git commit -m "fix: x"`, proof.Policy(), tree))
	if err != nil || len(res.Findings) != 1 || !strings.Contains(res.Findings[0].Message, "stages 2 files and 403 new lines") {
		t.Errorf("result = %+v, %v", res, err)
	}
}

func TestShapeIsUnmeasuredWhenANewFileCannotBeRead(t *testing.T) {
	t.Parallel()
	tree := worktree{files: map[string]string{"new.go": "unreadable"}, untracked: []string{"new.go"}}
	res, err := shell.Shape().Run(t.Context(), input(`git add new.go && git commit -m "fix: x"`, proof.Policy(), tree))
	if err != nil || res.Status.Kind != model.StatusUnmeasured || res.Status.Reason != "read new.go: path unreadable" {
		t.Errorf("result = %+v, %v", res, err)
	}
}

// TestShapeLeavesAFileOverTheBoundUnread counts a staged file over the read
// bound and leaves its lines unread: under the limits the rule names the
// file among the files it left unread instead of judging, and a command
// whose files alone break the limits is still a finding.
func TestShapeLeavesAFileOverTheBoundUnread(t *testing.T) {
	t.Parallel()
	tree := worktree{files: map[string]string{"data/big.txt": "too large", "data/a.go": "x\n"}, untracked: []string{"data/big.txt", "data/a.go"}}
	res, err := shell.Shape().Run(t.Context(), input(`git add data && git commit -m "fix: data"`, proof.Policy(), tree))
	if err != nil || res.Status.Kind != model.StatusMeasured || len(res.Findings) != 0 || !slices.Equal(res.Unread, []string{"data/big.txt"}) {
		t.Fatalf("under the limits = %+v, %v", res, err)
	}
	res, err = shell.Shape().Run(t.Context(), input(adds(41)+` data && git commit -m "fix: data"`, proof.Policy(), tree))
	if err != nil || len(res.Findings) != 1 || len(res.Unread) != 0 {
		t.Fatalf("over the file limit = %+v, %v", res, err)
	}
}
