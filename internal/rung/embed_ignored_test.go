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

// globEmbedder embeds into a string every text file of its assets
// directory, which holds one file in the index.
const globEmbedder = "package a\n\nimport _ \"embed\"\n\n// Message is the embedded text.\n//\n//go:embed assets/*.txt\nvar Message string\n"

// A file git ignores that an embed pattern of an index package names is
// a stated limit of the choice between the worktree and a copy: git status
// does not report it, so the Go steps read the worktree in place, where go
// embeds it beside the staged file. go vet accepts the string, so the
// default ladder lets the commit go; go test builds the package and
// refuses with the embed error and the off switch, which test_staged on
// shows the developer. The test pins both answers and the in place read.
//
//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitReadsInPlaceBesideAnIgnoredFileAnEmbedNames(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	for name, tc := range map[string]struct {
		ladder string
		code   int
		texts  []string
	}{
		"the default ladder": {"", 0, []string{"vet: go vet found nothing"}},
		"test_staged on": {"ladder: {test_staged: true}\n", 1, []string{
			"invalid go:embed: multiple files for type string",
			"fix the tests before the commit or switch test_staged off in golaw.yml",
		}},
	} {
		t.Run(name, func(t *testing.T) {
			d := ignoredEmbedDeps(t, tc.ladder)
			out := rung.RunPreCommit(t.Context(), d)
			if out.Code != tc.code {
				t.Fatalf("an ignored file an embed names = %+v", out)
			}
			for _, text := range tc.texts {
				if !contains(out, text) {
					t.Errorf("the answer does not name %q: %+v", text, out)
				}
			}
			if _, err := os.Stat(filepath.Join(d.State, "runner")); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("an ignored file an embed names keeps the worktree read in place: %v", err)
			}
		})
	}
}

// ignoredEmbedDeps stages the embedding package and one asset, writes a
// second asset the pattern names and excludes it through .git/info/exclude.
func ignoredEmbedDeps(t *testing.T, ladder string) rung.Dependencies {
	t.Helper()
	r := armedRepo(t, armedPolicy+ladder)
	r.Stage(map[string]string{"a/a.go": globEmbedder, "a/assets/one.txt": "one\n"})
	r.Write("a/assets/two.txt", "two\n")
	exclude := filepath.Join(r.GitDir(), "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(exclude), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exclude, []byte("a/assets/two.txt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return deps(t, r)
}
