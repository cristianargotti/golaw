package commits_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/commits"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestConventionalReadsTheSubject(t *testing.T) {
	t.Parallel()
	denies(t, commits.Conventional(), "conventional-subject", map[string]bool{
		"feat: add the reader\n":                    false,
		"fix(source): keep the index\n":             false,
		"feat(cmd/golaw)!: drop a flag\n":           false,
		"chore(.github): pin the actions\n":         false,
		"docs(README): say why\n":                   false,
		"revert: drop the cache\n\nbody\n":          false,
		"\n\nbuild: after blank lines\n":            false,
		"Update the reader\n":                       true,
		"fix:missing space\n":                       true,
		"fix: Capitalized description\n":            true,
		"fix: 404 page\n":                           true,
		"feature: add the reader\n":                 true,
		"Fix: upper case type\n":                    true,
		"fix(): empty scope\n":                      true,
		"fix(a b): spaced scope\n":                  true,
		"  fix: leading spaces\n":                   true,
		"fix: \n":                                   true,
		"WIP\n":                                     true,
		"Merge the two readers\n":                   true,
		"Revert the reader\n":                       true,
		"# a comment\nfix: the subject follows\n":   false,
		"fixup! Update the reader\n":                true,
		"fixup! \n":                                 true,
		"feat add the reader\n\nfix: in the body\n": true,
	})
}

func TestConventionalLeavesWhatGitWritesAndAutosquash(t *testing.T) {
	t.Parallel()
	denies(t, commits.Conventional(), "conventional-subject", map[string]bool{
		"fixup! feat: add the reader\n":                 false,
		"squash! fixup! fix: keep the index\n":          false,
		"amend! docs: say why\n\nbody\n":                false,
		"Merge branch 'topic' into main\n":              false,
		"Merge branch 'main' of example.com:org/repo\n": false,
		"Merge branches 'a' and 'b'\n":                  false,
		"Merge remote-tracking branch 'origin/main'\n":  false,
		"Merge tag 'v0.1.0'\n":                          false,
		"Merge commit '0123456'\n":                      false,
		"Merge pull request #12 from org/topic\n":       false,
		"Revert \"feat: add the reader\"\n\nThis reverts commit 0123456789abcdef0123456789abcdef01234567.\n": false,
		"Reapply \"feat: add the reader\"\n": false,
		"":                                   false,
		"# only a comment\n":                 false,
	})
}

func TestConventionalNamesTheSubjectAndItsLine(t *testing.T) {
	t.Parallel()
	res := judgeCommit(t, commits.Conventional(), model.Commit{Message: "\nUpdate the reader\n", Cleanup: model.CleanupVerbatim})
	if len(res.Findings) != 1 || res.Scope != "1 message" {
		t.Fatalf("result = %+v", res)
	}
	f := res.Findings[0]
	for _, want := range []string{`"Update the reader"`, "type(scope): description", "feat, fix, chore", "lower case letter"} {
		if !strings.Contains(f.Message, want) {
			t.Errorf("message %q lacks %q", f.Message, want)
		}
	}
	if f.Line != 2 || f.Path != "" || f.Severity != model.SeverityDeny {
		t.Errorf("finding = %+v", f)
	}
}
