package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/checks/shell"
	"github.com/cristianargotti/golaw/internal/hook/codex"
)

// The m4-vendor category (section 10.3 item 5), the decision a forbidden
// prefix carries, and the scope of the one install that writes the rules
// file (D12, D26).
const (
	m4VendorCategory = "m4-vendor"
	m4ExecPolicyRow  = "codex-execpolicy"
	m4RulesFile      = "golaw.rules"
	decisionForbid   = "forbidden"
	flagVersion      = "--version"
	flagScope        = "--scope"
	scopeProject     = "project"
)

// m4PrefixMatch is one prefix rule a command matched.
type m4PrefixMatch struct {
	MatchedPrefix []string `json:"matchedPrefix"`
	Decision      string   `json:"decision"`
	Justification string   `json:"justification"`
}

// m4MatchedRule is one entry of matchedRules.
type m4MatchedRule struct {
	PrefixRuleMatch *m4PrefixMatch `json:"prefixRuleMatch"`
}

// m4PolicyAnswer is the line codex execpolicy check prints (survey section
// 7.3).
type m4PolicyAnswer struct {
	MatchedRules []m4MatchedRule `json:"matchedRules"`
	Decision     string          `json:"decision"`
}

// m4VendorRows is the m4-vendor category: codex execpolicy check on the
// rules file of a project-scope install with the Codex CLI the adapter's
// grammar is pinned to. Without that CLI the row is skipped, required by
// a release run, whose CI installs it, so a skip never passes there.
func (s *suite) m4VendorRows(ctx context.Context, required bool) []row {
	path, reason := s.m4PinnedCodex(ctx)
	if reason != "" {
		return []row{{m4VendorCategory, m4ExecPolicyRow, statusSkip, reason, required}}
	}
	item := measuredM4(m4VendorCategory, m4ExecPolicyRow, s.m4ExecPolicy(ctx, path))
	if item.status == statusPass {
		item.detail = "codex-cli " + codex.PatchGrammar + " loads the rules of a project-scope install and forbids every prefix"
	}
	return []row{item}
}

// m4PinnedCodex is the Codex CLI on PATH when it answers exactly the pinned
// version, never another build of it such as an alpha release, or the
// reason the row is skipped.
func (s *suite) m4PinnedCodex(ctx context.Context) (string, string) {
	path, err := s.lookup(agentCodex)
	if err != nil {
		return "", "the Codex CLI is not on PATH"
	}
	got := s.execute(ctx, command{program: path, args: []string{flagVersion}, dir: s.root, env: s.env})
	want := "codex-cli " + codex.PatchGrammar
	if success(got) != nil || strings.TrimSpace(got.stdout) != want {
		return "", fmt.Sprintf("the Codex CLI on PATH answers %q, not the pinned %s", strings.TrimSpace(got.stdout), want)
	}
	return path, ""
}

// m4ExecPolicy checks the rules file of the row's project-scope install:
// a regular file, because Codex skips a link (survey section 7.1), whose
// every prefix codex execpolicy check forbids with the reason of its check,
// while a near miss matches no rule.
func (s *suite) m4ExecPolicy(ctx context.Context, codexPath string) error {
	rules, err := s.m4ProjectRules(ctx)
	if err != nil {
		return err
	}
	info, err := os.Lstat(rules)
	if err != nil {
		return fmt.Errorf("inspect the installed rules file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("the installed %s is not a regular file, which Codex would skip: %w", m4RulesFile, ErrAcceptance)
	}
	prefixes := shell.Prefixes()
	if len(prefixes) == 0 {
		return fmt.Errorf("the shell family names no prefix: %w", ErrAcceptance)
	}
	for _, prefix := range prefixes {
		argv := m4PrefixArgv(prefix)
		answer, checkErr := s.m4Check(ctx, codexPath, rules, argv)
		if checkErr != nil {
			return checkErr
		}
		if answer.Decision != decisionForbid || !m4Justified(answer, prefix.Reason) {
			return fmt.Errorf("%q is not forbidden with the reason of %s: %+v: %w", argv, prefix.Check, answer, ErrAcceptance)
		}
	}
	near, err := s.m4Check(ctx, codexPath, rules, []string{m2GitCommand, gitStatus})
	if err != nil {
		return err
	}
	if len(near.MatchedRules) != 0 || near.Decision == decisionForbid {
		return fmt.Errorf("the near miss git status matched %+v: %w", near, ErrAcceptance)
	}
	return nil
}

// m4ProjectRules installs Codex with --scope project in an enrolled
// repository of a home of the row's own and answers the rules file that
// install wrote: a user-scope install writes none (D12, D26), so the
// installation the suite prepared holds no rules file to check.
func (s *suite) m4ProjectRules(ctx context.Context) (string, error) {
	home, env, err := s.m4Home(m4ExecPolicyRow)
	if err != nil {
		return "", err
	}
	t := m4Target{home: home, binary: s.binary, env: env}
	if t.repo, err = s.m4Repository(ctx, t, "m4-"+m4ExecPolicyRow); err != nil {
		return "", err
	}
	args := []string{commandAgent, verbInstall, flagAgent, agentCodex, flagScope, scopeProject}
	got := s.execute(ctx, command{program: t.binary, args: args, dir: t.repo, env: t.env})
	if installErr := success(got); installErr != nil {
		return "", fmt.Errorf("agent install --agent codex --scope project: %w", installErr)
	}
	return m4ProjectRulesPath(t.repo), nil
}

// m4ProjectRulesPath is where a project-scope install writes the rules
// file, the .codex/rules of the repository.
func m4ProjectRulesPath(repo string) string {
	return filepath.Join(repo, codexDir, rulesWord, m4RulesFile)
}

// m4PrefixArgv is the command a prefix rule spells with the first
// alternative of every position.
func m4PrefixArgv(prefix shell.Prefix) []string {
	argv := make([]string, 0, len(prefix.Words))
	for _, alternatives := range prefix.Words {
		if len(alternatives) > 0 {
			argv = append(argv, alternatives[0])
		}
	}
	return argv
}

// m4Check runs codex execpolicy check on one command, no shell between.
func (s *suite) m4Check(ctx context.Context, codexPath, rules string, argv []string) (m4PolicyAnswer, error) {
	args := append([]string{"execpolicy", "check", "--rules", rules}, argv...)
	got := s.execute(ctx, command{program: codexPath, args: args, dir: s.root, env: s.env})
	if err := success(got); err != nil {
		return m4PolicyAnswer{}, fmt.Errorf("codex execpolicy check %s: %w", strings.Join(argv, " "), err)
	}
	var answer m4PolicyAnswer
	if err := json.Unmarshal([]byte(got.stdout), &answer); err != nil {
		return m4PolicyAnswer{}, fmt.Errorf("decode the execpolicy answer %q: %w", got.stdout, err)
	}
	return answer, nil
}

// m4Justified reports whether a forbidden prefix rule of the answer carries
// the reason.
func m4Justified(answer m4PolicyAnswer, reason string) bool {
	return slices.ContainsFunc(answer.MatchedRules, func(rule m4MatchedRule) bool {
		return rule.PrefixRuleMatch != nil && rule.PrefixRuleMatch.Decision == decisionForbid &&
			rule.PrefixRuleMatch.Justification == reason
	})
}
