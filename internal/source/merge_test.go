package source_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/source"
)

// conflicted is a repository whose branch side added theirs.md and changed
// shared.md, which main changed too, so a merge of side stages theirs.md
// and leaves shared.md unmerged; it answers the root and the commit of side.
func conflicted(t *testing.T) (string, string) {
	t.Helper()
	root := initRepo(t)
	write(t, root, "shared.md", "start\n")
	commitAll(t, root, "docs: start")
	gitRun(t, root, "checkout", "-q", "-b", "side")
	write(t, root, "shared.md", "side\n")
	write(t, root, "theirs.md", "theirs\n")
	side := commitAll(t, root, "docs: side")
	gitRun(t, root, "checkout", "-q", "main")
	write(t, root, "shared.md", "main\n")
	commitAll(t, root, "docs: main")
	return root, side
}

// TestChangesNamesTheMergeInProgress answers, beside what git status
// reports, the commit MERGE_HEAD names while a merge is in progress, none
// before it, with the paths the merge staged and those it left unmerged,
// and refuses a MERGE_HEAD it cannot read.
func TestChangesNamesTheMergeInProgress(t *testing.T) {
	t.Parallel()
	root, side := conflicted(t)
	repo, err := source.Discover(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if st, beforeErr := source.Changes(t.Context(), repo, nil); beforeErr != nil || st.Merge != "" {
		t.Fatalf("before the merge: %q, %v", st.Merge, beforeErr)
	}
	merge := exec.CommandContext(t.Context(), "git", "merge", "-q", "side")
	merge.Dir = root
	merge.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=tester", "GIT_AUTHOR_EMAIL=tester@example.com", "GIT_COMMITTER_NAME=tester", "GIT_COMMITTER_EMAIL=tester@example.com")
	if out, mergeErr := merge.CombinedOutput(); mergeErr == nil {
		t.Fatalf("the merge did not stop on its conflict: %s", out)
	}
	st, err := source.Changes(t.Context(), repo, nil)
	if err != nil || st.Merge != side || !st.Staged["theirs.md"] || st.Staged["shared.md"] || !st.Unmerged["shared.md"] || st.Unmerged["theirs.md"] {
		t.Fatalf("a merge in progress: merge %q, staged %v, unmerged %v, %v", st.Merge, st.Staged, st.Unmerged, err)
	}
	unreadable := source.Repo{Root: root, GitDir: filepath.Join(root, ".git", "MERGE_HEAD")}
	if _, err := source.Changes(t.Context(), unreadable, nil); err == nil {
		t.Fatal("a MERGE_HEAD that cannot be read answered no merge")
	}
}
