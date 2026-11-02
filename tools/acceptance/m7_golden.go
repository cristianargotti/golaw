package main

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/policy"
)

// The m7-golden row of M7.13 row 2: its category, the variable that names
// the directory of the pinned golden tools (contract 3.5), and the reason a
// golden step prints when it skips for a pinned tool that is absent.
const (
	m7GoldenCategory = "m7-golden"
	m7GoldenToolsKey = "GOLAW_GOLDEN_TOOLS"
	m7SkipReason     = "not found: run golaw setup --gen"
	m7GoldenTarget   = "golden-gen"
	m7GoFlagsKey     = "GOFLAGS"
	m7VerboseFlag    = "-v"
)

// m7GoldenEnv names the tools directory once and asks go test for every
// test line: a quiet run hides the reason of a step a missing pin skipped,
// and that reason is what turns the row into a SKIP instead of a PASS.
func m7GoldenEnv(env []string, tools string) []string {
	flags := m7VerboseFlag
	out := slices.DeleteFunc(slices.Clone(env), func(entry string) bool {
		key, value, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, m7GoFlagsKey) {
			flags = strings.TrimSpace(value + " " + m7VerboseFlag)
			return true
		}
		return strings.EqualFold(key, m7GoldenToolsKey)
	})
	return append(out, m7GoldenToolsKey+"="+tools, m7GoFlagsKey+"="+flags)
}

// m7JudgeGolden reads the golden-gen run: a failed step fails the row even
// when the exit hides it, and a step skipped for a missing pin is a
// required SKIP naming the tool and the pin, so it never passes silently.
func m7JudgeGolden(got result) error {
	if got.err != nil {
		return fmt.Errorf("run make %s: %w", m7GoldenTarget, got.err)
	}
	lines := strings.Split(strings.ReplaceAll(got.stdout+"\n"+got.stderr, "\r\n", "\n"), "\n")
	for i, line := range lines {
		if strings.Contains(line, "--- FAIL") {
			return fmt.Errorf("make %s: %s: %s: %w", m7GoldenTarget, strings.TrimSpace(line), m7FailedOutput(lines[i+1:]), ErrAcceptance)
		}
	}
	if got.code != 0 {
		return fmt.Errorf("make %s: exit %d: %w", m7GoldenTarget, got.code, ErrAcceptance)
	}
	for _, line := range lines {
		if strings.Contains(line, m7SkipReason) {
			return m7Skip(strings.TrimSpace(line))
		}
	}
	return nil
}

// m7FailedLines bounds the lines of a failed test the row carries.
const m7FailedLines = 12

// m7FailedOutput joins the indented lines a failed test logged, the cause
// of the failure, so a row that fails only under load names it.
func m7FailedOutput(after []string) string {
	var out []string
	for _, line := range after {
		if !strings.HasPrefix(line, " ") || len(out) == m7FailedLines {
			break
		}
		out = append(out, strings.TrimSpace(line))
	}
	return strings.Join(out, " | ")
}

// m7GoldenCases runs the golden tests in the kit checkout, as the M2
// timing row runs its measurement: they are the kit's own tests.
func m7GoldenCases() []m7Case {
	return []m7Case{{name: "kinds", gen: true, run: (*m7Suite).m7GoldenKinds}}
}

// m7GoldenKinds runs the golden-gen target with the tools directory setup
// --gen fills over the shared data home.
func (s *m7Suite) m7GoldenKinds(ctx context.Context) error {
	tools, err := policy.ToolsDir(runtime.GOOS, func(key string) string {
		return environmentValue(s.m7GenEnv(), key)
	})
	if err != nil {
		return fmt.Errorf("resolve the golden tools directory: %w", err)
	}
	return m7JudgeGolden(s.execute(ctx, command{
		program: s.m2.Make, args: []string{m7GoldenTarget}, dir: s.root,
		env: m7GoldenEnv(s.env, tools), timeout: m2MeasurementTimeout,
	}))
}
