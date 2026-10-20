package rung

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
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
