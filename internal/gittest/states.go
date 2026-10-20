package gittest

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// The modes the helpers create directories and files with.
const (
	dirMode  = 0o750
	fileMode = 0o600
)

// Write creates or replaces a file at a slash separated path under the root.
func (r *Repo) Write(rel, content string) {
	r.t.Helper()
	path := filepath.Join(r.Root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		r.t.Fatalf("create the directory of %s: %v", rel, err)
		return
	}
	if err := os.WriteFile(path, []byte(content), fileMode); err != nil {
		r.t.Fatalf("write %s: %v", rel, err)
	}
}

// Remove deletes the file at a slash separated path under the root.
func (r *Repo) Remove(rel string) {
	r.t.Helper()
	if err := os.Remove(filepath.Join(r.Root, filepath.FromSlash(rel))); err != nil {
		r.t.Fatalf("remove %s: %v", rel, err)
	}
}

// Stage writes the files and stages them with git add, in path order.
func (r *Repo) Stage(files map[string]string) {
	r.t.Helper()
	paths := slices.Sorted(maps.Keys(files))
	if len(paths) == 0 {
		return
	}
	for _, rel := range paths {
		r.Write(rel, files[rel])
	}
	r.Git(append([]string{"add", "--"}, paths...)...)
}

// Commit writes and stages the files and commits everything staged with
// the message, an empty commit when nothing is staged; it answers the
// commit id.
func (r *Repo) Commit(message string, files map[string]string) string {
	r.t.Helper()
	r.Stage(files)
	r.Git("commit", "-q", "--allow-empty", "-m", message)
	return r.Git(revParse, head)
}

// StageOnly stages one content of a file and then leaves another in the
// worktree: partial staging, and opposite contents of index and worktree.
func (r *Repo) StageOnly(rel, staged, worktree string) {
	r.t.Helper()
	r.Stage(map[string]string{rel: staged})
	r.Write(rel, worktree)
}

// StageDeletion stages the deletion of a committed file.
func (r *Repo) StageDeletion(rel string) {
	r.t.Helper()
	r.Git("rm", "-q", "--", rel)
}

// StageRename stages the move of a committed file to a new path.
func (r *Repo) StageRename(from, to string) {
	r.t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(r.Root, filepath.FromSlash(to))), dirMode); err != nil {
		r.t.Fatalf("create the directory of %s: %v", to, err)
		return
	}
	r.Git("mv", from, to)
}

// Branch commits the files on a new branch from HEAD, comes back to the
// branch it left and answers the tip of the new one.
func (r *Repo) Branch(name string, files map[string]string) string {
	r.t.Helper()
	current := r.Git("symbolic-ref", "--short", "HEAD")
	r.Git("checkout", "-q", "-b", name)
	tip := r.Commit("chore: change on "+name, files)
	r.Git("checkout", "-q", current)
	return tip
}

// MergeInProgress starts git merge --no-commit --no-ff with the options
// and the branch and leaves it in progress; a conflict stays as git leaves
// it, and a merge that does not start fails the test.
func (r *Repo) MergeInProgress(branch string, args ...string) {
	r.t.Helper()
	merge := append(append([]string{"merge", "--no-commit", "--no-ff"}, args...), branch)
	_, mergeErr := r.run(nil, merge...)
	if _, err := r.run(nil, revParse, "-q", "--verify", "MERGE_HEAD"); err != nil {
		r.t.Fatalf("git %s left no merge in progress: %v", strings.Join(merge, " "), mergeErr)
	}
}

// Worktree adds a linked worktree on a new branch under the temporary
// directory of the test and answers it.
func (r *Repo) Worktree(branch string) *Repo {
	r.t.Helper()
	root := filepath.Join(realDir(r.t), "worktree")
	r.Git("worktree", "add", "-q", "-b", branch, root)
	return &Repo{Root: root, t: r.t}
}

// AlternateIndex answers the path of a new index file outside the git
// directory that holds what HEAD holds.
func (r *Repo) AlternateIndex() string {
	r.t.Helper()
	index := filepath.Join(realDir(r.t), "index")
	r.GitEnv([]string{"GIT_INDEX_FILE=" + index}, "read-tree", head)
	return index
}
