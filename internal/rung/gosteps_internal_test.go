package rung

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/source"
)

// A person who reached the worktree through a linked directory leaves PWD
// naming the link, and a tool takes PWD for its working directory when it
// names the same directory: golangci-lint 2.14.0 then drops every issue of
// --new-from-patch. The tools that read the tree run with PWD naming its
// real path, after the user environment, so the last value wins.
func TestToolsOfTheTreeSeeItsRealPath(t *testing.T) {
	t.Parallel()
	p := &precommit{deps: Dependencies{Environ: []string{"PATH=/bin", "PWD=/linked/work", "GIT_DIR=/work/.git"}}}
	tree := goTree{set: runner.AffectedSet{Packages: []string{"example.test/m/a"}, Patterns: []string{"./a"}}, dir: "/real/work"}
	want := []string{"PATH=/bin", "PWD=/linked/work", "PWD=/real/work"}
	if got := p.treeEnv(tree); !slices.Equal(got, want) {
		t.Errorf("tree environment = %v, want %v", got, want)
	}
	if c := p.goCommand(tree, "vet"); c.Dir != tree.dir || !slices.Equal(c.Env, want) || !slices.Equal(c.Args, []string{"vet", "./a"}) {
		t.Errorf("go command = %+v", c)
	}
}

// A person may set diff.noprefix, which drops the a/ and b/ of a patch, or
// diff.mnemonicPrefix, which renames them; golangci-lint finds the staged
// lines of a file through those prefixes and keeps no issue it cannot place,
// so the staged patch names them whatever the settings say.
func TestStagedPatchCarriesThePrefixesTheLinterReads(t *testing.T) {
	t.Parallel()
	for _, setting := range []string{"diff.noprefix", "diff.mnemonicPrefix"} {
		r := repository(t, "")
		r.Git("config", setting, "true")
		r.Stage(map[string]string{"a/a.go": "package a\n"})
		gitDir := r.GitDir()
		if err := os.MkdirAll(filepath.Join(gitDir, "golaw"), 0o700); err != nil {
			t.Fatal(err)
		}
		p := &precommit{deps: Dependencies{Dir: r.Root, Environ: gittest.Isolated()}, repo: source.Repo{GitDir: gitDir}}
		path, err := p.patch(t.Context())
		if err != nil {
			t.Fatalf("%s: %v", setting, err)
		}
		data, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(data), "\n+++ b/a/a.go\n") {
			t.Errorf("%s: patch %q, %v", setting, data, err)
		}
	}
}

// A git that cannot list the ignored files leaves the rung without the
// answer it needs to choose the tree, which is a failure of the step and
// never a tree read by guess.
func TestIgnoredGoNamesTheGitThatCannotAnswer(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	d := Dependencies{Dir: outside, Environ: append(gittest.Isolated(), "GIT_CEILING_DIRECTORIES="+filepath.Dir(outside))}
	if _, err := ignoredGo(t.Context(), d); !errors.Is(err, source.ErrGit) {
		t.Errorf("a directory outside every repository = %v", err)
	}
	d.Dir = filepath.Join(outside, "absent")
	if _, err := ignoredGo(t.Context(), d); err == nil || errors.Is(err, source.ErrGit) {
		t.Errorf("a git that cannot start = %v", err)
	}
}
