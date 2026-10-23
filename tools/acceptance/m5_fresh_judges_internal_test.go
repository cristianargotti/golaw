package main

import (
	"path/filepath"
	"slices"
	"testing"
)

// The tests below give the fresh machine rows the wrong candidates the
// simulated one never plays: an init or an agent install that prints the
// one line of D8 and still leaves a file in the clone or enrolls it on
// the second machine, so the claim that a refusal writes nothing is seen
// failing.

// m5FreshWrongStep runs the fresh row named name with reply answering
// first, and requires it to fail.
func m5FreshWrongStep(t *testing.T, name, kind string, reply m5Reply) {
	t.Helper()
	for _, c := range m5FreshCases() {
		if c.name != name {
			continue
		}
		s := m2TestSuite(t)
		scope := s.m5Scope(m5FreshCategory, name+"-"+kind)
		scope.execute = m5Script(t, scope, append([]m5Reply{reply}, m5FreshReplies(t)...)...).Execute
		if err := c.run(scope, t.Context()); err == nil {
			t.Errorf("%s passed over %s", name, kind)
		}
		return
	}
	t.Fatalf("no fresh row is named %s", name)
}

func TestM5FreshRowsRefuseARefusalThatWrites(t *testing.T) {
	t.Parallel()
	refusal := result{code: 2, stderr: "golaw: profile fixture is not installed: run " + m5InstallCommand + "\n"}
	leaves := func(t *testing.T, c command) {
		t.Helper()
		checkedWrite(t, filepath.Join(c.dir, "note.md"), "ok\n")
	}
	initApply := []string{commandInit, flagApply}
	agentInstall := []string{m5AgentCommand, m5InstallSub}
	for _, row := range []struct {
		name, kind string
		reply      m5Reply
	}{
		{"init-apply", "a file left", m5Reply{prefix: initApply, once: true, answer: m5After(leaves, refusal)}},
		{"init-apply", "the clone enrolled", m5Reply{prefix: initApply, once: true, answer: m5After(m5FakeEnroll, refusal)}},
		{"agent-install", "a file left", m5Reply{prefix: agentInstall, once: true, answer: m5After(leaves, refusal)}},
		{"agent-install", "the clone enrolled", m5Reply{prefix: agentInstall, once: true, answer: m5After(m5FakeEnroll, refusal)}},
	} {
		m5FreshWrongStep(t, row.name, row.kind, row.reply)
	}
}

// TestM5CatalogsRequireTheFreshRows holds the line of m5Catalogs that
// makes -m5 and -release require the fresh machine rows, right after the
// adoption rows, which no test above reaches.
func TestM5CatalogsRequireTheFreshRows(t *testing.T) {
	t.Parallel()
	categories := make([]string, 0, len(m5Catalogs()))
	for _, catalog := range m5Catalogs() {
		categories = append(categories, catalog.category)
		if catalog.category == m5FreshCategory && !slices.Equal(m5Names(catalog.cases), m5Names(m5FreshCases())) {
			t.Fatalf("the fresh catalog = %q", m5Names(catalog.cases))
		}
	}
	at := slices.Index(categories, m5FreshCategory)
	if at < 1 || categories[at-1] != m5AdoptionCategory {
		t.Fatalf("the categories = %q, want %s right after %s", categories, m5FreshCategory, m5AdoptionCategory)
	}
}
