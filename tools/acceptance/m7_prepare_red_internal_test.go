package main

import (
	"strings"
	"testing"
)

// TestM7GeneratorsSkipSetupAfterARefusedAdoption: an init the tools
// repository refuses stops the build before setup runs, and the cause names
// the adoption, so a row reports the step that failed.
func TestM7GeneratorsSkipSetupAfterARefusedAdoption(t *testing.T) {
	t.Parallel()
	s := newM7Suite(m2TestSuite(t))
	setups := 0
	s.execute = m5Script(t, s.suite,
		m5Reply{prefix: []string{commandInit}, once: true, answer: func(*testing.T, command) result {
			return result{code: 2, stderr: "golaw: exit 2\n"}
		}},
		m5Reply{prefix: []string{m6SetupCommand, m7SetupGen}, answer: func(*testing.T, command) result {
			setups++
			return result{}
		}},
	).Execute
	err := s.m7Generators(t.Context())
	if err == nil || !strings.Contains(err.Error(), "adopt the generator tools repository") || setups != 0 {
		t.Fatalf("generators over a refused init = %v after %d setup runs", err, setups)
	}
}
