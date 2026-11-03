package main

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/rulesdoc"
)

func TestRulesIndexHasOneLinePerEmbeddedRule(t *testing.T) {
	t.Parallel()
	data, err := golaw.Docs.ReadFile("docs/rules.md")
	if err != nil {
		t.Fatal(err)
	}
	ta := newTestApp(t)
	if code := ta.run(t.Context(), []string{"rules"}); code != exitClean {
		t.Fatalf("exit = %d, stderr = %q", code, ta.stderr.String())
	}
	lines := strings.Split(strings.TrimSuffix(ta.stdout.String(), "\n"), "\n")
	if want := len(rulesdoc.ParseRules(data)); len(lines) != want || want < 70 {
		t.Fatalf("index has %d lines, the page %d rules", len(lines), want)
	}
	if !strings.Contains(ta.stdout.String(), "\ncommit.shape: A commit other than feat") {
		t.Fatalf("stdout = %q", ta.stdout.String())
	}
}

func TestRulesPrintsTheMatchingRowsInFull(t *testing.T) {
	t.Parallel()
	ta := newTestApp(t)
	if code := ta.run(t.Context(), []string{"rules", "commit.shape"}); code != exitClean {
		t.Fatalf("exit = %d, stderr = %q", code, ta.stderr.String())
	}
	want := "commit.shape rules/4.12@1 needs none: A commit other than feat adding more than 40 new files or 400 new lines"
	if got := ta.stdout.String(); !strings.HasPrefix(got, want) || strings.Count(got, "\n") != 1 {
		t.Fatalf("stdout = %q", got)
	}
	ta = newTestApp(t)
	ta.run(t.Context(), []string{"rules", "goast"})
	for line := range strings.SplitSeq(strings.TrimSuffix(ta.stdout.String(), "\n"), "\n") {
		if !strings.HasPrefix(line, "goast.") {
			t.Fatalf("family line = %q", line)
		}
	}
}

func TestRulesChecklistSection(t *testing.T) {
	t.Parallel()
	ta := newTestApp(t)
	if code := ta.run(t.Context(), []string{"rules", "--checklist", "reviewer"}); code != exitClean {
		t.Fatalf("exit = %d, stderr = %q", code, ta.stderr.String())
	}
	got := ta.stdout.String()
	if !strings.HasPrefix(got, "## Reviewer\n") || strings.Contains(got, "## Conditional checks") {
		t.Fatalf("stdout = %q", got)
	}
}

func TestRulesRefusals(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		args []string
		want string
	}{
		{[]string{"rules", "nothing.here"}, `no rule matches "nothing.here"; golaw rules lists every id`},
		{[]string{"rules", "--checklist", "nope"}, "the names are builder, "},
		{[]string{"rules", "--checklist", "reviewer", "goast"}, "give a rule id or --checklist, not both"},
		{[]string{"rules", "a", "b"}, "accepts at most 1 arg"},
	} {
		ta := newTestApp(t)
		if code := ta.run(t.Context(), row.args); code != exitFailure || !strings.Contains(ta.stderr.String(), row.want) {
			t.Errorf("%v: exit = %d, stderr = %q", row.args, code, ta.stderr.String())
		}
	}
}
