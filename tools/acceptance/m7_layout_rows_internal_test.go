package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// m7DetectedPolicy is the policy init writes over the hub tree.
const m7DetectedPolicy = "golaw: 1\ngenerate:\n  layout:\n    adapter_sql: internal/adapters/out/{package}\n" +
	"    adapter_http: internal/adapters/out/{package}\n    handler: internal/adapters/in/{package}\n" +
	"  third_party:\n    sqlc_out: internal/adapters/out/postgres/sqlcgen\n"

// m7InitWrites answers init by writing policy into the repository.
func m7InitWrites(policy string) m5Reply {
	return m5Reply{prefix: []string{commandInit}, answer: m5After(func(t *testing.T, c command) {
		t.Helper()
		m2Write(t, filepath.Join(c.dir, policyFile), policy)
	}, result{})}
}

// m7RunCase runs one case of catalog by name over replies.
func m7RunCase(t *testing.T, cases []m7Case, name string, replies ...m5Reply) error {
	t.Helper()
	s := newM7Suite(m2TestSuite(t))
	s.execute = m5Script(t, s.suite, replies...).Execute
	for _, c := range cases {
		if c.name == name {
			return c.run(s, t.Context())
		}
	}
	t.Fatalf("no case %s", name)
	return nil
}

func TestM7LayoutRowsJudgeWhatInitAnswers(t *testing.T) {
	t.Parallel()
	named := result{code: 2, stderr: "golaw: internal/adapters/out (11 packages) and internal/adapters/outbound (1 packages) both hold hand written adapters: set generate.layout.adapter_sql and generate.layout.adapter_http in golaw.yml\n"}
	for _, tc := range []struct {
		name, row string
		replies   []m5Reply
		ok        bool
	}{
		{"near the detected layout", "detected-layout", []m5Reply{m7InitWrites(m7DetectedPolicy)}, true},
		{"red the outbound tree chosen", "detected-layout", []m5Reply{m7InitWrites(strings.ReplaceAll(m7DetectedPolicy, "adapters/out/", "adapters/outbound/"))}, false},
		{"red init refused", "detected-layout", []m5Reply{{prefix: []string{commandInit}, answer: m5Answer(result{code: 2})}}, false},
		{"red no policy written", "detected-layout", []m5Reply{{prefix: []string{commandInit}, answer: m5Answer(result{})}}, false},
		{"near both candidates named", "ambiguous-layout", []m5Reply{{prefix: []string{commandInit}, answer: m5Answer(named)}}, true},
		{"red a silent choice", "ambiguous-layout", []m5Reply{m7InitWrites(m7DetectedPolicy)}, false},
	} {
		if err := m7RunCase(t, m7LayoutCases(), tc.row, tc.replies...); (err == nil) != tc.ok {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
}

func TestM7AdoptedStopsBeforeInitOnlyForItsOwnSteps(t *testing.T) {
	t.Parallel()
	s := newM7Suite(m2TestSuite(t))
	s.base = filepath.Join(t.TempDir(), "base")
	m2Write(t, s.base, "x")
	s.execute = m5Script(t, s.suite).Execute
	if _, _, err := s.m7Adopted(t.Context(), "blocked", "go", m7HubTree(false)); err == nil {
		t.Fatal("a repository under a file was adopted")
	}
	s = newM7Suite(m2TestSuite(t))
	s.execute = m5Script(t, s.suite, m5Reply{program: s.git, prefix: []string{commandInit}, answer: m5Answer(result{code: 128})}).Execute
	for _, run := range []func(*m7Suite, context.Context) error{(*m7Suite).m7DetectedLayoutCase, (*m7Suite).m7AmbiguousLayoutCase} {
		if err := run(s, t.Context()); err == nil || !strings.Contains(err.Error(), "exit 128") {
			t.Fatalf("a refused git init = %v", err)
		}
	}
}

// m7MapText is a map of tree as golaw map prints it: the open holes first.
func m7MapText(holeIDs, ports []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "golaw map layout: port internal/application/{area}\n")
	for _, id := range holeIDs {
		fmt.Fprintf(&b, "hole open internal/application/%s/holes.go:4 %s return the answer\n", strings.Split(id, ".")[0], id)
	}
	for _, p := range ports {
		fmt.Fprintf(&b, "port %s internal/application/%s/ports.go: Read0; Read1\n", p, strings.Split(p, ".")[0])
	}
	return b.String()
}

func TestM7MapRowsJudgeWhatMapPrints(t *testing.T) {
	t.Parallel()
	_, holeIDs, ports := m7MapTree()
	good := m7MapText(holeIDs, ports)
	mapReply := func(stdout string, code int) m5Reply {
		return m5Reply{prefix: []string{"map"}, answer: m5Answer(result{stdout: stdout, code: code})}
	}
	for _, tc := range []struct {
		name, row string
		replies   []m5Reply
		ok        bool
	}{
		{"near holes first", "holes-first", []m5Reply{m7InitWrites(m7DetectedPolicy), mapReply(good, 0)}, true},
		{"near within the budget", "budget", []m5Reply{m7InitWrites(m7DetectedPolicy), mapReply(good, 0)}, true},
		{"red a port first", "holes-first", []m5Reply{m7InitWrites(m7DetectedPolicy), mapReply("port area5.Port1\n"+good, 0)}, false},
		{"red over the budget", "budget", []m5Reply{m7InitWrites(m7DetectedPolicy), mapReply(good+strings.Repeat("x", 8004), 0)}, false},
		{"red map refused", "budget", []m5Reply{m7InitWrites(m7DetectedPolicy), mapReply("", 2)}, false},
		{"red init refused", "holes-first", []m5Reply{{prefix: []string{commandInit}, answer: m5Answer(result{code: 2})}}, false},
	} {
		if err := m7RunCase(t, m7MapCases(), tc.row, tc.replies...); (err == nil) != tc.ok {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
}

func TestM7MapStopsAtTheFirstFailingStep(t *testing.T) {
	t.Parallel()
	s := newM7Suite(m2TestSuite(t))
	s.execute = m5Script(t, s.suite, m5Reply{program: s.git, prefix: []string{commandInit}, answer: m5Answer(result{code: 128})}).Execute
	if _, _, _, err := s.m7Map(t.Context(), "refused"); err == nil {
		t.Fatal("a map ran over a refused git init")
	}
	s = newM7Suite(m2TestSuite(t))
	s.execute = m5Script(t, s.suite, m7InitWrites(m7DetectedPolicy), m5Reply{program: s.git, prefix: []string{gitAdd}, answer: m5Answer(result{code: 128})}).Execute
	if _, _, _, err := s.m7Map(t.Context(), "uncommitted"); err == nil {
		t.Fatal("a map ran over an adoption git refused to commit")
	}
}
