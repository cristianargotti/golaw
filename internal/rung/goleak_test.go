package rung_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// Two files of package a: one that starts a goroutine, and one that may.
const (
	starts = "package a\n\n// Start starts a goroutine.\nfunc Start() { go println() }\n"
	quiet  = "package a\n\n// Quiet starts nothing.\nfunc Quiet() {}\n"
	louder = "package a\n\n// Quiet starts nothing.\nfunc Quiet() {}\n\n// Louder starts a goroutine.\nfunc Louder() { go println() }\n"
)

// goast.goleak judges a package, and the finding sits at the first start
// a run meets: a goroutine added to a sibling file of a package whose
// committed file already starts one without a goleak test is the
// violation the base holds, so the commit goes; the first goroutine of a
// package is the commit's own and is refused.
//
//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitReadsAGoleakFindingByItsPackage(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	inherited := armedRepo(t, armedPolicy+"capabilities: [go]\n")
	inherited.Commit("feat: start the worker", map[string]string{"a/a.go": starts, "a/b.go": quiet})
	inherited.Stage(map[string]string{"a/b.go": louder})
	if out := rung.RunPreCommit(t.Context(), deps(t, inherited)); out.Code != 0 || contains(out, "goast.goleak") {
		t.Errorf("a goroutine beside the one the base starts = %+v", out)
	}
	first := armedRepo(t, armedPolicy+"capabilities: [go]\n")
	first.Commit("feat: add the package", map[string]string{"a/a.go": quiet, "a/b.go": "package a\n"})
	first.Stage(map[string]string{"a/b.go": louder})
	out := rung.RunPreCommit(t.Context(), deps(t, first))
	if out.Code != 1 || !contains(out, "a/b.go:7:17 deny goast.goleak package a starts goroutines and no test of the package verifies with goleak") {
		t.Errorf("the first goroutine of a package = %+v", out)
	}
}

// A committed file of the package over the read bound may start the
// goroutines a goroutine added beside it inherits, so the finding is no
// proof of the commit: the rung names the file and lets the commit go.
// The first goroutine of a package whose oversized neighbor lies in
// another package is still the commit's own and is refused.
//
//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitLeavesToThePipelineAGoleakFindingBesideAnUnreadFile(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	oversized := starts + strings.Repeat("//\n", 3<<20)
	inherited := armedRepo(t, armedPolicy+"capabilities: [go]\n")
	inherited.Commit("feat: start the worker", map[string]string{"a/a.go": oversized, "a/b.go": quiet})
	inherited.Stage(map[string]string{"a/b.go": louder})
	out := rung.RunPreCommit(t.Context(), deps(t, inherited))
	if out.Code != 0 || contains(out, "deny goast.goleak") || !contains(out, "over the read bound, not judged here: a/a.go; the pipeline judges them") {
		t.Errorf("a goroutine beside an oversized file of its package = %+v", out)
	}
	first := armedRepo(t, armedPolicy+"capabilities: [go]\n")
	first.Commit("feat: add the packages", map[string]string{"a/b.go": quiet, "c/c.go": strings.Replace(oversized, "package a", "package c", 1)})
	first.Stage(map[string]string{"a/b.go": louder})
	out = rung.RunPreCommit(t.Context(), deps(t, first))
	if out.Code != 1 || !contains(out, "a/b.go:7:17 deny goast.goleak package a starts goroutines and no test of the package verifies with goleak") {
		t.Errorf("the first goroutine of a package beside an oversized file of another = %+v", out)
	}
}
