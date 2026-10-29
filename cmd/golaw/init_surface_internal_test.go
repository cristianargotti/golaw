package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/toolchain"
	"github.com/cristianargotti/golaw/internal/toolchaintest"
)

// surfaceInitApp is initApp over the surface binding, so a fake tool at
// its layout path runs during init.
func surfaceInitApp(t *testing.T) (testApp, string) {
	t.Helper()
	ta, _ := initApp(t)
	b, tools, _ := surfaceBinding(t)
	ta.bind = func() binding { return b }
	return ta, tools
}

func readPolicyFile(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "golaw.yml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestInitWritesTheStrictSetUnlessParityIsAsked(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		args []string
		want string
	}{
		"default":        {nil, "surface: { rules: strict }\n"},
		"parity by flag": {[]string{"--surface-rules", "parity"}, "surface: { rules: parity }\n"},
		"strict by flag": {[]string{"--surface-rules", "strict"}, "surface: { rules: strict }\n"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ta, _ := surfaceInitApp(t)
			root := checkout(t)
			args := append([]string{"init", "--apply"}, tc.args...)
			if code := ta.run(t.Context(), append(args, root)); code != exitClean {
				t.Fatalf("exit = %d: %s", code, ta.stderr.String())
			}
			if got := readPolicyFile(t, root); strings.Count(got, "surface:") != 1 || !strings.Contains(got, tc.want) {
				t.Errorf("golaw.yml = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestInitRefusesAnUnknownSurfaceSet(t *testing.T) {
	t.Parallel()
	ta, _ := surfaceInitApp(t)
	if code := ta.run(t.Context(), []string{"init", "--surface-rules", "lenient", checkout(t)}); code != exitFailure || !strings.Contains(ta.stderr.String(), "parity or strict") {
		t.Errorf("exit = %d, stderr %q", code, ta.stderr.String())
	}
}

func TestInitKeepsAReviewedPolicyWithoutTheFlag(t *testing.T) {
	t.Parallel()
	ta, _ := surfaceInitApp(t)
	root := checkout(t)
	reviewed := renderedPolicy(t)
	if err := os.WriteFile(filepath.Join(root, "golaw.yml"), reviewed, 0o600); err != nil {
		t.Fatal(err)
	}
	if code := ta.run(t.Context(), []string{"init", "--apply", root}); code != exitClean {
		t.Fatalf("exit = %d: %s", code, ta.stderr.String())
	}
	if got := readPolicyFile(t, root); strings.Contains(got, "surface:") {
		t.Errorf("a reviewed policy gained the surface line without the flag: %q", got)
	}
}

func TestInitCountsWhatTheStrictSetFindsInTheExistingTree(t *testing.T) {
	t.Parallel()
	ta, tools := surfaceInitApp(t)
	hadolint, _ := toolchain.Lookup("hadolint")
	toolchaintest.Install(t, tools, hadolint, runtime.GOOS, runtime.GOARCH, runnertest.Behavior{Stdout: lastUserFinding, Code: 1})
	repo := gittest.New(t)
	repo.Commit("build: start", map[string]string{"go.mod": "module example.test/m\n\ngo 1.27\n", "Dockerfile": "FROM scratch\nCOPY app /app\nUSER 0\n"})
	if code := ta.run(t.Context(), []string{"init", repo.Root}); code != exitClean {
		t.Fatalf("exit = %d: %s", code, ta.stderr.String())
	}
	out := ta.stdout.String()
	for _, want := range []string{
		"surface: o conjunto strict encontra 1 problema no que o repositório já tem:",
		"  hadolint DL3002: 1",
		"surface: sem contagem para checkov (rode golaw setup --surface)",
		"golaw init --surface-rules parity",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	parity, _ := surfaceInitApp(t)
	if code := parity.run(t.Context(), []string{"init", "--surface-rules", "parity", repo.Root}); code != exitClean || strings.Contains(parity.stdout.String(), "conjunto strict") {
		t.Errorf("a parity init counted the strict set: %d %q", code, parity.stdout.String())
	}
}

func TestDebtLinesCountInTheLanguageOfThePolicy(t *testing.T) {
	t.Parallel()
	finding := func(rule string) model.Finding {
		return model.Finding{Check: "lint.hadolint", Rule: rule, Path: "Dockerfile", Line: 1}
	}
	run := &engine.Run{Outcomes: []engine.Outcome{
		{Spec: model.Spec{ID: "lint.hadolint"}, Result: model.Result{Findings: []model.Finding{finding("hadolint/DL3002"), finding("hadolint/DL3007")}}},
		{Spec: model.Spec{ID: "lint.checkov"}, Result: model.Result{Status: model.Unmeasured("absent")}},
		{Spec: model.Spec{ID: "lint.inventory"}, Result: model.Result{Status: model.Unmeasured("unjudged")}},
	}}
	got := strings.Join(debtLines(run, false), "\n")
	for _, want := range []string{
		"surface: the strict set finds 2 issues in what the repository already holds:",
		"  hadolint DL3002: 1\n  hadolint DL3007: 1",
		"run golaw init --surface-rules parity",
		"surface: no count for checkov (run golaw setup --surface)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("lines lack %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "inventory") {
		t.Errorf("the inventory outcome was named as a tool:\n%s", got)
	}
	if one := debtHeader(1, false); !strings.Contains(one, "1 issue in") {
		t.Errorf("one finding = %q", one)
	}
	if many := debtHeader(3, true); !strings.Contains(many, "3 problemas") {
		t.Errorf("three findings = %q", many)
	}
	if lines := debtLines(&engine.Run{}, true); len(lines) != 0 {
		t.Errorf("a clean run printed %q", lines)
	}
}

func TestInitUpdateRefusesTheSurfaceFlag(t *testing.T) {
	t.Parallel()
	ta, _ := surfaceInitApp(t)
	if code := ta.run(t.Context(), []string{"init", "--update", "--surface-rules", "strict", checkout(t)}); code != exitFailure || !strings.Contains(ta.stderr.String(), "--surface-rules") {
		t.Errorf("exit = %d, stderr %q", code, ta.stderr.String())
	}
}
