package main

import (
	"context"
	"fmt"
	"strings"
)

const (
	m3PolicyCategory = "m3-policy"
	m3Baseline       = "baseline"
	m3ServiceCase    = "service"
	// m3TailLines bounds the output a failed make leaves in a row's detail,
	// enough to name the failing target and the finding before it.
	m3TailLines = 12
)

func m3PolicyNames() []string {
	return []string{m3Baseline}
}

func (s *suite) policyM3(ctx context.Context, name string) error {
	if name != m3Baseline {
		return fmt.Errorf("unknown M3 policy case %q: %w", name, ErrAcceptance)
	}
	repo, err := s.m3ServiceRepository(ctx, m3ServiceCase, []string{"go"})
	if err != nil {
		return err
	}
	return s.m3PassesCI(ctx, repo)
}

// m3PassesCI runs the two steps the generated CI ends with: make ci, then
// git diff --exit-code, so a target that rewrites a tracked file fails the
// row as it fails the pipeline.
func (s *suite) m3PassesCI(ctx context.Context, repo string) error {
	got := s.m3Make(ctx, repo, "ci")
	if got.err != nil {
		return fmt.Errorf("execute make ci: %w", got.err)
	}
	if got.code != 0 {
		return fmt.Errorf("make ci exit %d: %s: %w", got.code, m3Tail(got), ErrAcceptance)
	}
	if err := s.gitSteps(ctx, repo, []string{"diff", "--exit-code"}); err != nil {
		return fmt.Errorf("make ci changed a tracked file: %w", err)
	}
	return nil
}

// m3Tail is the end of a process output, the part that names what failed.
func m3Tail(got result) string {
	text := strings.TrimSpace(strings.ReplaceAll(got.stdout+"\n"+got.stderr, "\r\n", "\n"))
	lines := strings.Split(text, "\n")
	if len(lines) > m3TailLines {
		lines = lines[len(lines)-m3TailLines:]
	}
	return strings.Join(lines, "\n")
}
