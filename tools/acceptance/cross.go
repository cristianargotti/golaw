package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

const (
	categoryPolicy = "policy"
	caseCross      = "cross-location"
)

// located is a fixture with the working directory of the session that
// sends it, which the cross rows keep apart from the repository.
type located struct {
	f   fixture
	dir string
}

// crossCases are the rows of A3 that set the session apart from the
// target: a dash written and a secret read in the enrolled repository from
// an unenrolled directory, both denied, and a dash written from the
// enrolled repository into the agent's own plans, outside every
// repository, allowed.
func (s *suite) crossCases(repo, session string) []located {
	read := fixture{
		name: "foreign-cwd-read", event: eventPre, tool: toolRead, want: wantDeny,
		input: map[string]any{keyFilePath: filepath.Join(repo, ".env")},
	}
	plans := filepath.Join(environmentValue(s.env, "CLAUDE_CONFIG_DIR"), "plans", "plan.md")
	return []located{
		{writeFixture("foreign-cwd-write", filepath.Join(repo, pathNote), textDash, wantDeny), session},
		{read, session},
		{writeFixture("outside-target-write", plans, textDash, wantAllow), repo},
	}
}

// crossRows replay the cross cases through the installed invocation, the
// hook running in the session's directory as the agent runs it.
func (s *suite) crossRows(ctx context.Context) []row {
	repo, err := s.repository(ctx, caseCross, true)
	session := filepath.Join(s.base, dirRepos, caseCross+"-session")
	if err == nil {
		if mkErr := os.MkdirAll(session, dirMode); mkErr != nil {
			err = fmt.Errorf("create the session directory: %w", mkErr)
		}
	}
	cases := s.crossCases(repo, session)
	rows := make([]row, 0, len(cases))
	for i := range cases {
		c := &cases[i]
		got := err
		if got == nil {
			got = s.judgeFixture(ctx, c.dir, c.f, c.f.want)
		}
		rows = append(rows, measured(categoryPolicy, c.f.name, got))
	}
	return rows
}
