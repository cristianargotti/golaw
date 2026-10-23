package source_test

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
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
	// A MERGE_HEAD that is a directory cannot be read on any system, where
	// a path through a file reads as absent on Windows.
	gitDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(gitDir, "MERGE_HEAD"), 0o700); err != nil {
		t.Fatal(err)
	}
	unreadable := source.Repo{Root: root, GitDir: gitDir}
	if _, err := source.Changes(t.Context(), unreadable, nil); err == nil {
		t.Fatal("a MERGE_HEAD that cannot be read answered no merge")
	}
}

// looseObjects counts the files under the object directory of a
// repository, its packs included, so a test sees an object written there.
func looseObjects(t *testing.T, root string) int {
	t.Helper()
	count := 0
	err := filepath.WalkDir(filepath.Join(root, ".git", "objects"), func(_ string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			count++
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return count
}

// TestAutomaticMergeNamesTheBlobsOfTheMergeGitMakes merges main and side
// as git merges them by itself and names, at each path asked, the blob of
// that merge and the blob side holds: the conflict of shared.md in its
// markers beside the version of side, theirs.md as side brought it, a file
// only main holds beside no blob, and none for a path neither holds, from a
// linked worktree too; the repository receives no object, and a commit git
// does not hold is an error.
func TestAutomaticMergeNamesTheBlobsOfTheMergeGitMakes(t *testing.T) {
	t.Parallel()
	root, side := conflicted(t)
	write(t, root, "local.md", "local\n")
	commitAll(t, root, "docs: local")
	worktree := filepath.Join(t.TempDir(), "linked")
	gitRun(t, root, "worktree", "add", "-q", "--detach", worktree)
	objects := looseObjects(t, root)
	paths := []string{"shared.md", "theirs.md", "local.md", "none.md"}
	for _, dir := range []string{root, worktree} {
		repo, err := source.Discover(t.Context(), dir)
		if err != nil {
			t.Fatal(err)
		}
		blobs, err := source.AutomaticMerge(t.Context(), repo, repo.Head, side, paths)
		theirs, shared := gitRun(t, root, "rev-parse", side+":theirs.md"), gitRun(t, root, "rev-parse", side+":shared.md")
		conflict := blobs["shared.md"]
		if err != nil || len(conflict) != 2 || conflict[1] != shared || conflict[0] == "" || conflict[0] == shared ||
			conflict[0] == gitRun(t, root, "rev-parse", "HEAD:shared.md") ||
			!slices.Equal(blobs["theirs.md"], []string{theirs, theirs}) ||
			!slices.Equal(blobs["local.md"], []string{gitRun(t, root, "rev-parse", "HEAD:local.md"), ""}) ||
			!slices.Equal(blobs["none.md"], []string{"", ""}) {
			t.Fatalf("%s: blobs %v, %v", dir, blobs, err)
		}
	}
	if after := looseObjects(t, root); after != objects {
		t.Fatalf("the merge wrote %d objects into the repository", after-objects)
	}
	repo, err := source.Discover(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.AutomaticMerge(t.Context(), repo, repo.Head, strings.Repeat("0", 40), paths); err == nil {
		t.Fatal("a commit git does not hold answered a merge")
	}
}

// resolvedMerge is a repository whose merge commit at HEAD resolved the
// conflict of shared.md with bytes of its own, added mine.md, merged
// auto.md, which both sides changed apart, as git merges it by itself, and
// took theirs.md from side whole; it answers the root, the commit main held
// before the merge and the merge.
func resolvedMerge(t *testing.T) (string, string, string) {
	t.Helper()
	root := initRepo(t)
	write(t, root, "shared.md", "start\n")
	write(t, root, "auto.md", "a\nb\nc\n")
	commitAll(t, root, "docs: start")
	gitRun(t, root, "checkout", "-q", "-b", "side")
	write(t, root, "shared.md", "side\n")
	write(t, root, "auto.md", "a\nb\nside c\n")
	write(t, root, "theirs.md", "theirs\n")
	commitAll(t, root, "docs: side")
	gitRun(t, root, "checkout", "-q", "main")
	write(t, root, "shared.md", "main\n")
	write(t, root, "auto.md", "main a\nb\nc\n")
	before := commitAll(t, root, "docs: main")
	merge := exec.CommandContext(t.Context(), "git", "merge", "-q", "side")
	merge.Dir = root
	merge.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=tester", "GIT_AUTHOR_EMAIL=tester@example.com", "GIT_COMMITTER_NAME=tester", "GIT_COMMITTER_EMAIL=tester@example.com")
	if out, err := merge.CombinedOutput(); err == nil {
		t.Fatalf("the merge did not stop on its conflict: %s", out)
	}
	write(t, root, "shared.md", "resolved\n")
	write(t, root, "mine.md", "mine\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "--no-edit")
	return root, before, gitRun(t, root, "rev-parse", "HEAD")
}

// TestRemergedNamesWhatAMergeHoldsApartFromTheMergeGitMakes reads a merge
// commit two ways: git log names it as a merge with the paths it holds
// apart from every parent, the resolution, the new file and the file git
// merged by itself, and git log --remerge-diff with the paths it holds apart
// from the merge git makes of its parents by itself, the resolution and the
// new file, writing no object into the repository; a commit git does not
// hold is an error.
func TestRemergedNamesWhatAMergeHoldsApartFromTheMergeGitMakes(t *testing.T) {
	t.Parallel()
	root, before, merge := resolvedMerge(t)
	repo, err := source.Discover(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	commits, err := source.Commits(t.Context(), repo, before, repo.Head, 0)
	if err != nil || len(commits) != 2 || !commits[0].Merge || commits[0].Name != merge ||
		!slices.Equal(commits[0].Paths, []string{"auto.md", "mine.md", "shared.md"}) || commits[1].Merge {
		t.Fatalf("commits = %+v, %v", commits, err)
	}
	objects := looseObjects(t, root)
	remerged, err := source.Remerged(t.Context(), repo, []string{merge})
	if err != nil || len(remerged) != 1 || !slices.Equal(remerged[merge], []string{"mine.md", "shared.md"}) {
		t.Fatalf("remerged = %v, %v", remerged, err)
	}
	if after := looseObjects(t, root); after != objects {
		t.Fatalf("the remerge wrote %d objects into the repository", after-objects)
	}
	if _, err := source.Remerged(t.Context(), repo, []string{strings.Repeat("0", 40)}); err == nil {
		t.Fatal("a commit git does not hold answered a remerge")
	}
}
