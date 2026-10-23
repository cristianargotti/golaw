package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The tests below give the debt rows the wrong candidates the simulated
// one never plays, so every claim of a row is seen failing: a seed whose
// record lists nothing, a refused or a plain init that still rewrites the
// seeded policy, a rung that lets the broadening through, and a staged
// scan that refuses the entry that shrinks.

// m5WrongDebtStep runs the debt row named name with the reply built over
// its scope answering first, and requires it to fail.
func m5WrongDebtStep(t *testing.T, name, kind string, reply func(s *suite) m5Reply) {
	t.Helper()
	cases := m5AdoptionCases()
	at := slices.Index(m5Names(cases), name)
	if at < 0 {
		t.Fatalf("no adoption row is named %s", name)
	}
	s := m2TestSuite(t)
	scope := s.m5Scope(m5AdoptionCategory, name+"-"+kind)
	replies := append([]m5Reply{reply(scope)}, m5DebtReplies(t, scope)...)
	scope.execute = m5Script(t, scope, replies...).Execute
	if err := cases[at].run(scope, t.Context()); err == nil {
		t.Errorf("%s passed over %s", name, kind)
	}
}

// m5Broadened adds a second entry to the seeded policy of a command's
// checkout, the change a person may never make through init.
func m5Broadened(t *testing.T, c command) {
	t.Helper()
	path := filepath.Join(c.dir, policyFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	extra := "exceptions:\n  - { check: " + m5DashCheck + ", path: other.md, record: " + m5DebtRecord + ", until: 2027-04-19 }\n"
	broadened := strings.Replace(string(data), "exceptions:\n", extra, 1)
	if broadened == string(data) {
		t.Fatalf("the seeded policy holds no exception list: %q", data)
	}
	m2Write(t, path, broadened)
}

func TestM5DebtRowsRefuseAStepThatLies(t *testing.T) {
	t.Parallel()
	refused := result{code: 2, stderr: "golaw: init: " + m5SeededOnce + " (A24); remove entries from golaw.yml, never add them\n"}
	staged := result{code: 2, stderr: "golaw: scan: proposed golaw.yml " + m5Weakens + "[text.dashes at notes.md].until\n"}
	for _, row := range []struct {
		name, kind string
		reply      func(s *suite) m5Reply
	}{
		{"debt-seeded-once", "a record that lists nothing", func(s *suite) m5Reply {
			seed := m5DebtReplies(t, s)[0].answer
			return m5Reply{prefix: []string{commandInit, m5SeedFlag}, once: true, answer: func(t *testing.T, c command) result {
				t.Helper()
				got := seed(t, c)
				m2Write(t, filepath.Join(c.dir, filepath.FromSlash(m5DebtRecord)), "# Inherited debt\n")
				return got
			}}
		}},
		{"debt-seeded-once", "a refused seed that rewrites the policy", func(s *suite) m5Reply {
			seed := m5DebtReplies(t, s)[0].answer
			return m5Reply{prefix: []string{commandInit, m5SeedFlag}, answer: func(t *testing.T, c command) result {
				t.Helper()
				if _, err := os.Stat(filepath.Join(c.dir, policyFile)); err != nil {
					return seed(t, c)
				}
				m5Broadened(t, c)
				return refused
			}}
		}},
		{"debt-seeded-once", "a plain init that broadens the policy", func(*suite) m5Reply {
			return m5Reply{prefix: []string{commandInit, flagApply}, answer: m5After(m5Broadened, result{})}
		}},
		{"debt-broadening-refused", "a rung that lets the broadening through", func(*suite) m5Reply {
			return m5Reply{prefix: []string{commandHook, "pre-commit"}, answer: m5Answer(result{})}
		}},
		{"debt-broadening-refused", "a staged scan that refuses the shrinking entry", func(*suite) m5Reply {
			return m5Reply{prefix: []string{commandScan, m5StagedFlag}, answer: m5Answer(staged)}
		}},
	} {
		m5WrongDebtStep(t, row.name, row.kind, row.reply)
	}
}
