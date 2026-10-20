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

// stepLine is the line of one step of the pre-commit rung.
func stepLine(step, text string) string {
	return prefix(PreCommit) + step + ": " + text
}

// passed is a step that lets the commit go, with what it measured.
func passed(step, text string) Outcome {
	return Outcome{Lines: []string{stepLine(step, text)}}
}

// refusedBy is a step whose tool found what the commit introduces, with
// what the tool printed.
func refusedBy(step, text string, out runner.Output) Outcome {
	return Outcome{Code: codeFindings, Lines: slices.Concat([]string{stepLine(step, text)}, toolLines(out))}
}

// stepFailed is a step that could not judge.
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

// notShipped names the steps of section 7 that enter with the db and api
// templates, which this version does not ship.
func notShipped() Outcome {
	return Outcome{Lines: []string{
		stepLine("migrate-lint", "not applicable, this version ships no database template"),
		stepLine("openapi-breaking", "not applicable, this version ships no api template"),
		stepLine("generate-check", "not applicable, this version ships no generator template"),
	}}
}
