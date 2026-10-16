package main

import (
	"context"
	"strings"
	"testing"
)

// refusalBeforeCommit is the answer of agent install on a repository with
// commits whose reviewed policy is not committed yet.
func refusalBeforeCommit() result {
	return result{code: 2, stderr: "golaw: agent install: missing-policy: " + commitStep + ", then run agent install again\n"}
}

// existingAnswers answers the first agent install of the existing
// repository row with first, every scan there with scan, and every other
// command the way base does.
func existingAnswers(base executor, first, scan result) executor {
	installs := 0
	return func(ctx context.Context, c command) result {
		if !strings.HasSuffix(c.dir, caseExisting) || len(c.args) == 0 {
			return base(ctx, c)
		}
		switch c.args[0] {
		case "agent":
			installs++
			if installs == 1 {
				return first
			}
		case "scan":
			return scan
		}
		return base(ctx, c)
	}
}

func TestExistingRepositoryRow(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	base := s.execute
	s.execute = existingAnswers(base, refusalBeforeCommit(), result{})
	assertPass(t, s.existingRow(t.Context()))
	for name, answers := range map[string]executor{
		"installed before the commit": existingAnswers(base, result{}, result{}),
		"refused without the step":    existingAnswers(base, result{code: 2, stderr: "golaw: agent install: missing-policy\n"}, result{}),
		"refusal that never ran":      existingAnswers(base, result{err: ErrAcceptance}, result{}),
		"scan with findings":          existingAnswers(base, refusalBeforeCommit(), result{code: 1}),
		"git that fails": func(ctx context.Context, c command) result {
			if c.program == s.git {
				return result{code: 128, stderr: "fatal\n"}
			}
			return existingAnswers(base, refusalBeforeCommit(), result{})(ctx, c)
		},
	} {
		s.execute = answers
		if got := s.existingRow(t.Context()); got.status != statusFail || got.category != categoryInstallation || got.name != caseExisting {
			t.Errorf("%s: %+v", name, got)
		}
	}
}
