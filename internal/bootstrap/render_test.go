package bootstrap_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
)

const renderedCore = `# golaw policy, written by golaw init. Every later change is a reviewed
# diff that may only tighten the policy in force: floors fall, sets shrink,
# nothing here weakens a rule (golaw.yml is read from HEAD, never from the
# working tree).
golaw: 1
kit: { version: 0.1.0 }
language: pt-BR
capabilities: [api, go]
layout: { main: root, decisions: docs/adr }
checks:
  sizes: { file_lines: 300, function_lines: 40 }
  coverage: { total: 80, package: 80, file: 80, min_statements: 20, file_exemptions: [] }
  terms: { tools: [], allow_paths: [] }
  lint:
    allowed_exclusions:
      - path: _test\.go
        linters: [funlen, mnd, goconst, dupl, gosec, noctx, bodyclose, gochecknoglobals, musttag]
  shell: { deny_push: false, deny_pr: false }
ladder: { pre_commit: true, commit_msg: true, pre_push: true, test_staged: false }
vulnerability_waivers: []
exceptions: []
managed: []
`

func goLayout() bootstrap.Layout {
	return bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/adr", Go: true}
}

func TestNormalizeSortsAddsGoAndRefusesTheUnknown(t *testing.T) {
	t.Parallel()
	in := bootstrap.Inputs{KitVersion: "0.1.0", Language: "pt-BR", Capabilities: []string{"db", "api", "db"}, Layout: goLayout()}
	got, err := in.Normalize()
	if err != nil || !slices.Equal(got.Capabilities, []string{"api", "db", "go"}) {
		t.Fatalf("capabilities = %v, %v", got.Capabilities, err)
	}
	bad := map[string]bootstrap.Inputs{
		"no version": {Language: "pt-BR", Layout: goLayout()},
		"language":   {KitVersion: "0.1.0", Language: "portuguese", Layout: goLayout()},
		"capability": {KitVersion: "0.1.0", Language: "en", Capabilities: []string{"web"}, Layout: goLayout()},
		"no layout":  {KitVersion: "0.1.0", Language: "en"},
	}
	for name, in := range bad {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := in.Normalize(); !errors.Is(err, model.ErrUsage) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestRenderIsTheGoldenPolicyAndDeterministic(t *testing.T) {
	t.Parallel()
	in, err := bootstrap.Inputs{KitVersion: "0.1.0", Language: "pt-BR", Capabilities: []string{"api"}, Layout: goLayout()}.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	first, err := bootstrap.Render(in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := bootstrap.Render(in)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != renderedCore || string(second) != renderedCore {
		t.Fatalf("rendered:\n%s", first)
	}
}

func TestRenderWithoutCapabilitiesIsAnEmptyList(t *testing.T) {
	t.Parallel()
	in, err := bootstrap.Inputs{KitVersion: "0.0.0-devel", Language: "en", Layout: bootstrap.Layout{Main: bootstrap.MainCmd, Decisions: "docs/decisions"}}.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	got, err := bootstrap.Render(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"kit: { version: 0.0.0-devel }", "language: en", "capabilities: []", "layout: { main: cmd, decisions: docs/decisions }"} {
		if !strings.Contains(string(got), want+"\n") {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestDiffShowsEveryLineOnceAndIsEmptyWhenEqual(t *testing.T) {
	t.Parallel()
	if got := bootstrap.Diff([]byte("a\nb\n"), []byte("a\nb\n")); got != "" {
		t.Fatalf("equal diff = %q", got)
	}
	created := bootstrap.Diff(nil, []byte("golaw: 1\nlanguage: pt-BR\n"))
	wantCreated := "--- /dev/null\n+++ golaw.yml\n@@ -1,0 +1,2 @@\n+golaw: 1\n+language: pt-BR\n"
	if created != wantCreated {
		t.Fatalf("created diff = %q", created)
	}
	changed := bootstrap.Diff([]byte("a\nb\nc\n"), []byte("a\nx\nc\nd\n"))
	wantChanged := "--- golaw.yml\n+++ golaw.yml\n@@ -1,3 +1,4 @@\n a\n-b\n+x\n c\n+d\n"
	if changed != wantChanged {
		t.Fatalf("changed diff = %q", changed)
	}
	newline := map[string]struct{ old, current, want string }{
		"added":   {"a", "a\n", "@@ -1,1 +1,1 @@\n-a\n\\ No newline at end of file\n+a\n"},
		"removed": {"a\nb\n", "a\nb", "@@ -1,2 +1,2 @@\n a\n-b\n+b\n\\ No newline at end of file\n"},
		"neither": {"x\na", "x\nb", "@@ -1,2 +1,2 @@\n x\n-a\n\\ No newline at end of file\n+b\n\\ No newline at end of file\n"},
	}
	for name, tc := range newline {
		if got := bootstrap.Diff([]byte(tc.old), []byte(tc.current)); got != "--- golaw.yml\n+++ golaw.yml\n"+tc.want {
			t.Fatalf("final newline %s: diff = %q", name, got)
		}
	}
	if got := bootstrap.Diff([]byte("a"), []byte("a")); got != "" {
		t.Fatalf("equal without a final newline = %q", got)
	}
}
