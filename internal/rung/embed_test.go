package rung_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// A package that embeds a file of a subdirectory, which go vet reads as an
// input of the package.
const (
	embedder = "package a\n\nimport _ \"embed\"\n\n// Message is the embedded text.\n//\n//go:embed assets/message.txt\nvar Message string\n"
	asset    = "a/assets/message.txt"
)

// An embedded file of a subdirectory is a Go input: when the worktree
// lacks the file the index holds, the Go steps read a copy of the index and
// the valid commit goes; when the index lacks the file the worktree holds,
// the copy shows go vet the commit it records, which the embed breaks.
//
//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitReadsTheIndexForAnEmbeddedFile(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	r := armedRepo(t, armedPolicy)
	r.Stage(map[string]string{"a/a.go": embedder, asset: "hello\n"})
	r.Remove(asset)
	d := deps(t, r)
	if out := rung.RunPreCommit(t.Context(), d); out.Code != 0 || !contains(out, "vet: go vet found nothing") || !emptyDir(t, treesOf(d)) {
		t.Errorf("an embedded file staged and gone from the worktree = %+v", out)
	}
	r.Write(asset, "changed only in the worktree\n")
	if d := deps(t, r); rung.RunPreCommit(t.Context(), d).Code != 0 || !emptyDir(t, treesOf(d)) {
		t.Error("an embedded file changed only in the worktree is read in place")
	}
	r.Git("rm", "-q", "-f", "--cached", "--", asset)
	out := rung.RunPreCommit(t.Context(), deps(t, r))
	if out.Code != 1 || !contains(out, "pattern assets/message.txt: no matching files found") {
		t.Errorf("an embedded file the index lacks and the worktree holds = %+v", out)
	}
}

// A file of a subdirectory no embed pattern names is no Go input, so a
// change of it only in the worktree keeps the worktree read in place.
//
//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitReadsInPlaceBesideAFileNoEmbedNames(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	r := armedRepo(t, armedPolicy)
	r.Commit("feat: embed the message", map[string]string{"a/a.go": embedder, asset: "hello\n", "a/docs/notes.md": "why\n"})
	r.Stage(map[string]string{"a/a.go": embedder + "\n// Other is another value.\nvar Other = 1\n"})
	r.Write("a/docs/notes.md", "changed\n")
	d := deps(t, r)
	if out := rung.RunPreCommit(t.Context(), d); out.Code != 0 || !contains(out, "vet: go vet found nothing") {
		t.Errorf("a file no embed names, changed in the worktree = %+v", out)
	}
	if _, err := os.Stat(filepath.Join(d.State, "runner")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a file no embed names keeps the worktree read in place: %v", err)
	}
}

// treesOf is the directory the copies of the index go under.
func treesOf(d rung.Dependencies) string {
	return filepath.Join(d.State, "runner", "trees")
}
