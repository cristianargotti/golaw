package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestConfigExplainPrintsThePolicyWithItsSources(t *testing.T) {
	t.Parallel()
	ta := newTestApp(t)
	var root string
	ta.explain = func(_ context.Context, path string, now time.Time) (string, error) {
		root = path
		return "language: pt-BR (golaw.yml)\nclock: " + now.Format(time.DateOnly) + "\n", nil
	}
	if got := ta.run(t.Context(), []string{"config", "explain", "repo"}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	if root != "repo" || ta.stdout.String() != "language: pt-BR (golaw.yml)\nclock: 2026-10-14\n" {
		t.Fatalf("root = %q, stdout = %q", root, ta.stdout.String())
	}
}

func TestConfigExplainFailureIsExitTwo(t *testing.T) {
	t.Parallel()
	ta := newTestApp(t)
	ta.explain = func(context.Context, string, time.Time) (string, error) { return "", errBoom }
	if got := ta.run(t.Context(), []string{"config", "explain"}); got != exitFailure {
		t.Fatalf("exit = %d", got)
	}
	if got := ta.stderr.String(); got != "golaw: config explain: boom\n" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestConfigAloneIsAUsageError(t *testing.T) {
	t.Parallel()
	ta := newTestApp(t)
	if got := ta.run(t.Context(), []string{"config"}); got != exitFailure {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	if !strings.Contains(ta.stderr.String(), "usage: config needs a subcommand") {
		t.Fatalf("stderr = %q", ta.stderr.String())
	}
}
