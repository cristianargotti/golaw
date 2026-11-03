package check_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/check"
	"github.com/cristianargotti/golaw/internal/gittest"
)

// Changes names the committed, staged, unstaged, untracked and deleted
// paths since the fork, and nothing ignored or unchanged.
func TestChangesNameEveryStateOfTheWorktree(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	fork := repo.Commit("feat: the base", map[string]string{
		"go.mod": "module example.test/m\n\ngo 1.27\n", "keep.go": "package m\n", "gone.go": "package m\n",
		"edit.go": "package m\n", ".gitignore": "ignored.go\n",
	})
	repo.Commit("feat: a commit", map[string]string{"committed.go": "package m\n"})
	repo.Stage(map[string]string{"staged.go": "package m\n"})
	repo.Write("edit.go", "package m\n\nvar E = 1\n")
	repo.Write("sub/ação.go", "package sub\n")
	repo.Write("ignored.go", "package m\n")
	repo.Remove("gone.go")
	got, err := check.Changes(t.Context(), repo.Root, fork, gittest.Isolated())
	want := []string{"committed.go", "edit.go", "gone.go", "staged.go", "sub/ação.go"}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("changes = %q, %v, want %q", got, err, want)
	}
}

func TestChangesRefuseAForkGitCannotRead(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Commit("feat: the base", map[string]string{"a.go": "package m\n"})
	if _, err := check.Changes(t.Context(), repo.Root, "nowhere", gittest.Isolated()); !errors.Is(err, check.ErrGit) {
		t.Fatalf("err = %v, want ErrGit", err)
	}
}

func TestScopeSelectsAndAnswersChanged(t *testing.T) {
	t.Parallel()
	scope := check.Scope{Base: "main", Paths: []string{"a.go", "db/migrations/001.sql", "db/seed.sql", "docs/x.md"}}
	if got := scope.Select(check.GoFile); !slices.Equal(got, []string{"a.go"}) {
		t.Errorf("Go files = %q", got)
	}
	if got := scope.Select(check.Migration); !slices.Equal(got, []string{"db/migrations/001.sql"}) {
		t.Errorf("migrations = %q", got)
	}
	for _, tc := range []struct {
		scope check.Scope
		path  string
		want  bool
	}{
		{scope, "docs/x.md", true},
		{scope, "b.go", false},
		{check.Scope{Whole: true}, "b.go", true},
	} {
		if got := tc.scope.Changed(tc.path); got != tc.want {
			t.Errorf("Changed(%s) = %v", tc.path, got)
		}
	}
}
