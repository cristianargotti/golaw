package rung

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
)

// maxToolLines bounds the lines of tool output a step repeats; the rest is
// counted.
const maxToolLines = 200

// stepLine names the rung and the step in every line, because git prints
// the output of every hook with no frame around it, and a person or an
// agent who reads one line alone must know which step to fix.
func stepLine(step, text string) string {
	return prefix(PreCommit) + step + ": " + text
}

// passed prints a line even for a step that lets the commit go or skips,
// because a silent step could not be told from one that never ran.
func passed(step, text string) Outcome {
	return Outcome{Lines: []string{stepLine(step, text)}}
}

// refusedBy is a step whose tool found what the commit introduces, with
// what the tool printed.
func refusedBy(step, text string, out runner.Output) Outcome {
	return Outcome{Code: codeFindings, Lines: slices.Concat([]string{stepLine(step, text)}, toolLines(out))}
}

// stepFailed answers the failure code and not the findings one: a step
// that could not judge found nothing wrong in the commit, so its line
// names what broke, which is what the person fixes, not the change.
func stepFailed(step string, err error) Outcome {
	return Outcome{Code: codeFailure, Lines: []string{stepLine(step, err.Error())}}
}

// broke is a step whose tool answered an exit status it never answers for
// a finding, with what the tool printed.
func broke(step, tool string, out runner.Output) Outcome {
	text := fmt.Sprintf("%s failed with exit status %d", tool, out.Code)
	return Outcome{Code: codeFailure, Lines: slices.Concat([]string{stepLine(step, text)}, toolLines(out))}
}

// toolLines is what a tool printed, its output then its errors, at most
// maxToolLines lines and the count of the rest.
func toolLines(out runner.Output) []string {
	text := strings.TrimSpace(strings.TrimSpace(string(out.Stdout)) + "\n" + strings.TrimSpace(string(out.Stderr)))
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if len(lines) > maxToolLines {
		return append(lines[:maxToolLines], fmt.Sprintf("... and %d more lines", len(lines)-maxToolLines))
	}
	return lines
}

// join adds one outcome to another: the worse code, a failure before a
// finding, and the lines in order.
func join(a, b Outcome) Outcome {
	return Outcome{Code: max(a.Code, b.Code), Lines: slices.Concat(a.Lines, b.Lines)}
}

// verified finds the pinned tool of the name on PATH at its pinned
// version, its probe run with the user environment.
func verified(ctx context.Context, name string, env []string) (string, error) {
	tool, ok := runner.LookupTool(name)
	if !ok {
		return "", fmt.Errorf("%w: %s is no pinned tool", model.ErrUsage, name)
	}
	path, err := runner.Verify(ctx, tool, env)
	if err != nil {
		return "", fmt.Errorf("verify %s: %w", name, err)
	}
	return path, nil
}
