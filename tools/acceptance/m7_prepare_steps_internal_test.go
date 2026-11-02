package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestM7PreparationStopsAtTheStepThatFailed: Go tools that cannot be
// installed and a scanner proxy that cannot be built each stop the
// preparation, so no case starts from an environment without them.
func TestM7PreparationStopsAtTheStepThatFailed(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		fault func(t *testing.T, s *suite)
		want  string
	}{
		{"red the tools prerequisites missing", func(_ *testing.T, s *suite) {
			s.m2Prerequisites = func(*suite, string, string) ([]string, error) { return nil, ErrAcceptance }
		}, ErrAcceptance.Error()},
		{"red the scanner source blocked", func(t *testing.T, s *suite) {
			t.Helper()
			m2Write(t, filepath.Join(s.base, "scanner-source"), "not a directory")
		}, ""},
	} {
		s := m2TestSuite(t)
		s.m2.Ready = false
		s.execute = newM2Script(t, s, m3PrepareAnswer(t, s, m7CandidateVersion)).Execute
		tc.fault(t, s)
		err := s.prepareM7(t.Context())
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: prepare = %v", tc.name, err)
		}
		if s.m2.Overlay != nil {
			t.Fatalf("%s: the overlay was set over a failed preparation", tc.name)
		}
	}
}

// TestM7GeneratorsNeedTheShellAfterTheTidyModule: a tidy tools module on a
// machine without a shell stops the build before setup --gen runs, so no
// row reads receipts that were never written.
func TestM7GeneratorsNeedTheShellAfterTheTidyModule(t *testing.T) {
	t.Parallel()
	s := newM7Suite(m2TestSuite(t))
	s.lookup = func(string) (string, error) { return "", ErrAcceptance }
	built := false
	s.execute = m5Script(t, s.suite,
		m6InitReply(),
		m5Reply{program: s.goTool, prefix: []string{"mod", "tidy"}, answer: m5Answer(result{})},
		m5Reply{prefix: []string{m6SetupCommand, m7SetupGen}, answer: func(*testing.T, command) result {
			built = true
			return result{}
		}},
	).Execute
	err := s.m7Generators(t.Context())
	if err == nil || built || !strings.Contains(err.Error(), "shell prerequisite") {
		t.Fatalf("generators without a shell = %v, setup --gen ran %t", err, built)
	}
}
