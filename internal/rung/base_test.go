package rung_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// A sentinel declared beside its use, which goast.sentinel-decl denies,
// and the same file cut short so it does not parse.
const (
	sentinel       = "package a\n\nimport \"errors\"\n\n// ErrGone is declared beside its use.\nvar ErrGone = errors.New(\"gone\")\n\n// Find answers the sentinel.\nfunc Find() error { return ErrGone }\n"
	sentinelBroken = "package a\n\nimport \"errors\"\n\n// ErrGone is declared beside its use.\nvar ErrGone = errors.New(\"gone\")\n\n// Find answers the sentinel.\nfunc Find() error { return ErrGone\n"
)

// A base the comparison cannot measure, as one that does not parse, cannot
// tell an inherited finding from a new one, and the person cannot fix the
// base through the commit: the rung names the checks it left to the
// pipeline and lets the commit that repairs the syntax go. A base that
// parses still tells them apart, so a new sentinel is refused.
//
//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitLeavesToThePipelineWhatABrokenBaseCannotTell(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	r := armedRepo(t, armedPolicy)
	r.Commit("feat: add the finder", map[string]string{"a/a.go": sentinelBroken})
	r.Stage(map[string]string{"a/a.go": sentinel})
	out := rung.RunPreCommit(t.Context(), deps(t, r))
	if out.Code != 0 || !contains(out, "golaw pre-commit: not judged against a base it could not measure, the pipeline judges them: goast.sentinel-decl (parse a/a.go") {
		t.Errorf("a commit that repairs the syntax of its base = %+v", out)
	}
	fresh := armedRepo(t, armedPolicy)
	fresh.Commit("feat: add the finder", map[string]string{"a/a.go": "package a\n\n// Find answers nothing.\nfunc Find() error { return nil }\n"})
	fresh.Stage(map[string]string{"a/a.go": sentinel})
	if out := rung.RunPreCommit(t.Context(), deps(t, fresh)); out.Code != 1 || !contains(out, "deny goast.sentinel-decl") {
		t.Errorf("a new sentinel over a base that parses = %+v", out)
	}
}
