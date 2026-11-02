package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestM7GeneratorsStopBeforeSetupGenAtEachFault: a tools repository that
// cannot be written, a git init refused and an adoption refused each stop
// the build before setup --gen runs, and the adoption names itself.
func TestM7GeneratorsStopBeforeSetupGenAtEachFault(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		mode func(t *testing.T, s *m7Suite) []m5Reply
		want string
	}{
		{"red a base that is a file", func(t *testing.T, s *m7Suite) []m5Reply {
			t.Helper()
			file := filepath.Join(t.TempDir(), "base")
			if err := writeFile(file, []byte("x"), fileMode); err != nil {
				t.Fatal(err)
			}
			s.shared.base = file
			return nil
		}, ""},
		{"red a git init refused", func(_ *testing.T, s *m7Suite) []m5Reply {
			return []m5Reply{{program: s.git, prefix: []string{commandInit}, answer: m5Answer(result{code: 128})}}
		}, ""},
		{"red an adoption refused", func(*testing.T, *m7Suite) []m5Reply {
			return []m5Reply{{prefix: []string{commandInit}, answer: m5Answer(result{code: 2, stderr: "golaw: exit 2\n"})}}
		}, "adopt the generator tools repository"},
		{"red the tools module untidy", func(_ *testing.T, s *m7Suite) []m5Reply {
			return []m5Reply{m6InitReply(), {program: s.goTool, prefix: []string{"mod", "tidy"}, answer: m5Answer(result{code: 1, stderr: "go: refused\n"})}}
		}, "tidy the module"},
	} {
		s := newM7Suite(m2TestSuite(t))
		built := false
		replies := append(tc.mode(t, s), m5Reply{prefix: []string{m6SetupCommand, m7SetupGen}, answer: func(*testing.T, command) result {
			built = true
			return result{}
		}})
		s.execute = m5Script(t, s.suite, replies...).Execute
		err := s.m7Generators(t.Context())
		if err == nil || built || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: generators = %v, setup --gen ran %v", tc.name, err, built)
		}
	}
}

// TestM7PreparationNeedsMake: the fresh repository rows run its make ci,
// so a machine without make stops the preparation by name.
func TestM7PreparationNeedsMake(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	s.execute = newM2Script(t, s, m3PrepareAnswer(t, s, m7CandidateVersion)).Execute
	lookup := s.lookup
	s.lookup = func(name string) (string, error) {
		if name == m3MakeName {
			return "", ErrAcceptance
		}
		return lookup(name)
	}
	if err := s.prepareM7(t.Context()); err == nil || !strings.Contains(err.Error(), "locate make") {
		t.Fatalf("prepare without make = %v", err)
	}
}
