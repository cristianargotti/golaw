package source_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/source"
)

// TestHashObjectsNamesTheBytesAsGitStoresThem asks git once for the blob
// the bytes of each path would be stored as: a file checked out with
// carriage returns under core.autocrlf is named as the blob without them
// that HEAD holds, a plain file as its own bytes, a name one line cannot
// carry is left out, and a directory outside git is an error.
func TestHashObjectsNamesTheBytesAsGitStoresThem(t *testing.T) {
	t.Parallel()
	root := initRepo(t)
	write(t, root, "lf.md", "one\ntwo\n")
	write(t, root, "plain.md", "three\n")
	commitAll(t, root, "one")
	gitRun(t, root, "config", "core.autocrlf", "true")
	write(t, root, "lf.md", "one\r\ntwo\r\n")
	repo, err := source.Discover(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	hashes, err := source.HashObjects(t.Context(), repo, []string{"lf.md", "plain.md", "line\nbreak", `"quoted.md`})
	head := func(path string) string { return strings.TrimSpace(gitRun(t, root, "rev-parse", "HEAD:"+path)) }
	if err != nil || len(hashes) != 2 || hashes["lf.md"] != head("lf.md") || hashes["plain.md"] != head("plain.md") {
		t.Fatalf("HashObjects = %v, %v", hashes, err)
	}
	if none, err := source.HashObjects(t.Context(), repo, nil); err != nil || len(none) != 0 {
		t.Fatalf("no path = %v, %v", none, err)
	}
	if _, err := source.HashObjects(t.Context(), source.Repo{Root: t.TempDir()}, []string{"lf.md"}); err == nil {
		t.Fatal("a directory outside git answered hashes")
	}
}
