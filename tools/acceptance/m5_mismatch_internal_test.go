package main

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/profile"
)

func TestM5ScanRefusalIsOneLineWithItsFragments(t *testing.T) {
	t.Parallel()
	line := "golaw: scan: load the policy of /r: invalid policy: profile fixture: digest sha256:1 of the installed bundle differs from its stamp sha256:2: run " + m5InstallCommand + "\n"
	rows := []struct {
		name string
		got  result
		pass bool
	}{
		{"near the one line", result{code: 2, stderr: line}, true},
		{"red exit 1", result{code: 1, stderr: line}, false},
		{"red a document on stdout", result{code: 2, stdout: "{}", stderr: line}, false},
		{"red two lines", result{code: 2, stderr: line + line}, false},
		{"red no prefix", result{code: 2, stderr: strings.TrimPrefix(line, "golaw: ")}, false},
		{"red no remedy", result{code: 2, stderr: "golaw: scan: digest differs\n"}, false},
		{"red not run", result{err: ErrAcceptance}, false},
	}
	for _, row := range rows {
		if err := m5ScanRefusal(row.got, "digest", m5InstallCommand); (err == nil) != row.pass {
			t.Errorf("%s: %v", row.name, err)
		}
	}
}

func TestM5BrokenBundlesAreRefusedByTheLoader(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	dirs, err := s.m5Dirs()
	if err != nil {
		t.Fatal(err)
	}
	ref, err := m5InvalidBundle(testRoot(t), dirs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Read(profile.Dir(dirs.Config, m5Name)); !errors.Is(err, profile.ErrBundle) || !strings.Contains(err.Error(), m5UnknownKey) {
		t.Fatalf("the invalid bundle read as %v", err)
	}
	if ref.Name != m5Name || ref.Version != m5Version || !strings.HasPrefix(ref.Digest, "sha256:") {
		t.Fatalf("the pin of the invalid bundle = %+v", ref)
	}
	if err := m5CopyBundle(testRoot(t), profile.Dir(dirs.Config, m5Name)); err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Read(profile.Dir(dirs.Config, m5Name)); err != nil {
		t.Fatalf("the fixture bundle read as %v", err)
	}
	if err := m5AppendRules(dirs); err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Read(profile.Dir(dirs.Config, m5Name)); !errors.Is(err, profile.ErrDigest) || !errors.Is(err, model.ErrPolicy) {
		t.Fatalf("the tampered bundle read as %v", err)
	}
}

// m5MismatchReplies are the scans and the removal of each mismatch row:
// the refusal of D8 where the row expects one, a clean scan elsewhere.
func m5MismatchReplies() map[string][]m5Reply {
	digest := result{code: 2, stderr: "golaw: scan: profile fixture: digest sha256:1 of the installed bundle differs from its stamp sha256:2: run " + m5InstallCommand + "\n"}
	schema := result{code: 2, stderr: "golaw: scan: profile fixture: rules.yml: field " + m5UnknownKey + " not found; " + m5NewerKit + ", otherwise fix the key\n"}
	stale := result{code: 2, stderr: "golaw: scan: profile fixture: version is 1.2.3 in golaw.yml and 1.3.0 in the installed bundle: run " + m5InstallCommand + "\n"}
	return map[string][]m5Reply{
		"digest-mismatch": {
			{prefix: []string{commandScan}, once: true, answer: m5Answer(result{})},
			{prefix: []string{commandScan}, answer: m5Answer(digest)},
		},
		"schema-mismatch": {{prefix: []string{commandScan}, answer: m5Answer(schema)}},
		"stale-bundle": {
			{prefix: []string{m5ProfileCommand, "remove"}, answer: m5Answer(result{stdout: "Perfil fixture removido\n"})},
			{prefix: []string{commandScan}, once: true, answer: m5Answer(stale)},
			{prefix: []string{commandScan}, answer: m5Answer(result{})},
		},
	}
}

func TestM5MismatchRowsWithASimulatedCandidate(t *testing.T) {
	t.Parallel()
	for name, replies := range m5MismatchReplies() {
		c := m5ProfileCases()[slices.Index(m5Names(m5ProfileCases()), name)]
		m5Sweep(t, m5ProfileCategory, c, func(t *testing.T, _ *suite) []m5Reply {
			t.Helper()
			return m5ProfileReplies(t, replies...)
		})
		s := m2TestSuite(t)
		passing := s.m5Scope(m5ProfileCategory, name+"-passing")
		passing.execute = m5Script(t, passing, m5ProfileReplies(t, m5Reply{prefix: []string{commandScan}, answer: m5Answer(result{})},
			m5Reply{prefix: []string{m5ProfileCommand, "remove"}, answer: m5Answer(result{})})...).Execute
		if err := c.run(passing, t.Context()); err == nil {
			t.Fatalf("%s passed while every scan passed", name)
		}
	}
	if names := m5Names(m5ProfileCases()); !slices.Equal(names[3:], []string{"digest-mismatch", "schema-mismatch", "stale-bundle"}) {
		t.Fatalf("names = %q", names)
	}
}
