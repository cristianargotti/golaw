package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/shell"
	"github.com/cristianargotti/golaw/internal/hook/codex"
)

// m4PolicyFake is the pinned Codex CLI of the vendor row, the project-scope
// install the row runs, and the defect a red case gives them.
type m4PolicyFake struct {
	version              string
	reason               func(shell.Prefix) string
	nearForbidden        bool
	brokenRules, absent  bool
	failInstall          bool
	noRules, linkedRules bool
}

// answer is what the fake CLI prints for one command.
func (f m4PolicyFake) answer(t *testing.T, c command) result {
	t.Helper()
	if slices.Equal(c.args, []string{"--version"}) {
		return result{stdout: f.version + "\n"}
	}
	if f.brokenRules {
		return result{code: 1, stderr: "Error: failed to parse policy\n"}
	}
	argv := c.args[4:]
	var answer m4PolicyAnswer
	for _, prefix := range shell.Prefixes() {
		if slices.Equal(argv, m4PrefixArgv(prefix)) || f.nearForbidden {
			reason := prefix.Reason
			if f.reason != nil {
				reason = f.reason(prefix)
			}
			answer = m4PolicyAnswer{Decision: decisionForbid, MatchedRules: []m4MatchedRule{{PrefixRuleMatch: &m4PrefixMatch{
				MatchedPrefix: m4PrefixArgv(prefix), Decision: decisionForbid, Justification: reason,
			}}}}
		}
	}
	body, err := json.Marshal(answer)
	if err != nil {
		t.Fatal(err)
	}
	return result{stdout: string(body) + "\n"}
}

// install is the project-scope install the row runs: the rules file in
// the .codex/rules of the repository it runs in, none or a link when the
// case says so, and a failure for any other command line.
func (f m4PolicyFake) install(t *testing.T, c command) result {
	t.Helper()
	want := []string{commandAgent, verbInstall, flagAgent, agentCodex, flagScope, scopeProject}
	if f.failInstall || !slices.Equal(c.args, want) {
		return result{code: 1, stderr: "golaw: the install failed\n"}
	}
	if f.noRules {
		return result{}
	}
	rules := m4ProjectRulesPath(c.dir)
	if !f.linkedRules {
		checkedWrite(t, rules, "# golaw rules\n")
		return result{}
	}
	target := filepath.Join(t.TempDir(), "real.rules")
	checkedWrite(t, target, "# golaw rules\n")
	if err := os.MkdirAll(filepath.Dir(rules), dirMode); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, rules); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
	return result{}
}

// m4PolicySuite is a prepared suite with the fake CLI on its lookup and the
// fake project-scope install on its executor.
func m4PolicySuite(t *testing.T, fake m4PolicyFake) *suite {
	t.Helper()
	s := testSuite(t)
	base := s.execute
	s.lookup = func(name string) (string, error) {
		if fake.absent && name == agentCodex {
			return "", ErrAcceptance
		}
		return name, nil
	}
	s.execute = func(ctx context.Context, c command) result {
		switch {
		case c.program == agentCodex:
			return fake.answer(t, c)
		case len(c.args) > 0 && c.args[0] == commandAgent:
			return fake.install(t, c)
		}
		return base(ctx, c)
	}
	return s
}

func TestM4VendorRowChecksTheInstalledRules(t *testing.T) {
	t.Parallel()
	pinned := "codex-cli " + codex.PatchGrammar
	item := m4PolicySuite(t, m4PolicyFake{version: pinned}).m4VendorRows(t.Context(), true)[0]
	if item.status != statusPass || !item.required || !strings.Contains(item.detail, pinned) {
		t.Fatalf("the pinned CLI = %+v", item)
	}
	for _, tc := range []struct {
		name, status, want string
		fake               m4PolicyFake
	}{
		{"skip without the CLI", statusSkip, "not on PATH", m4PolicyFake{version: pinned, absent: true}},
		{"skip an alpha build of the version", statusSkip, "not the pinned", m4PolicyFake{version: pinned + "-alpha.10.2"}},
		{"red a reason the rule does not carry", statusFail, "is not forbidden", m4PolicyFake{version: pinned, reason: func(shell.Prefix) string { return "other" }}},
		{"red a near miss forbidden", statusFail, "near miss", m4PolicyFake{version: pinned, nearForbidden: true}},
		{"red a rules file Codex cannot load", statusFail, "codex execpolicy check", m4PolicyFake{version: pinned, brokenRules: true}},
		{"red a failed project install", statusFail, "--scope project", m4PolicyFake{version: pinned, failInstall: true}},
	} {
		for _, required := range []bool{false, true} {
			got := m4PolicySuite(t, tc.fake).m4VendorRows(t.Context(), required)[0]
			wantRequired := required || tc.status == statusFail
			if got.status != tc.status || got.required != wantRequired || !strings.Contains(got.detail, tc.want) {
				t.Errorf("%s, required %v: %+v", tc.name, required, got)
			}
		}
	}
}

func TestM4VendorRowReadsTheRulesOfItsProjectInstall(t *testing.T) {
	t.Parallel()
	s := m4PolicySuite(t, m4PolicyFake{version: "codex-cli " + codex.PatchGrammar})
	var installs, rules []string
	fake := s.execute
	s.execute = func(ctx context.Context, c command) result {
		switch {
		case len(c.args) > 0 && c.args[0] == commandAgent:
			installs = append(installs, c.dir)
		case c.program == agentCodex && len(c.args) > 3:
			rules = append(rules, c.args[3])
		}
		return fake(ctx, c)
	}
	if got := s.m4VendorRows(t.Context(), false)[0]; got.status != statusPass {
		t.Fatalf("the row = %+v", got)
	}
	repo := filepath.Join(s.base, dirRepos, "m4-"+m4ExecPolicyRow)
	want := m4ProjectRulesPath(repo)
	if !slices.Equal(installs, []string{repo}) || len(rules) == 0 ||
		slices.ContainsFunc(rules, func(path string) bool { return path != want }) {
		t.Fatalf("installs %q, rules %q, want the rules of %q", installs, rules, repo)
	}
}

func TestM4VendorRowRefusesARulesFileCodexWouldSkip(t *testing.T) {
	t.Parallel()
	pinned := "codex-cli " + codex.PatchGrammar
	for _, tc := range []struct {
		name, want string
		fake       m4PolicyFake
	}{
		{"red a project install that wrote no rules file", "inspect the installed rules file", m4PolicyFake{version: pinned, noRules: true}},
		{"red a linked rules file", "not a regular file", m4PolicyFake{version: pinned, linkedRules: true}},
	} {
		s := m4PolicySuite(t, tc.fake)
		// The near miss: a rules file at user scope, which golaw never
		// writes (D12, D26), must not stand in for the project one.
		checkedWrite(t, filepath.Join(s.home, codexDir, "rules", m4RulesFile), "# golaw rules\n")
		if got := s.m4VendorRows(t.Context(), false)[0]; got.status != statusFail || !strings.Contains(got.detail, tc.want) {
			t.Errorf("%s: %+v", tc.name, got)
		}
	}
}
