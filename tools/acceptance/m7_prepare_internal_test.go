package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestM7PreparationInstallsTheToolsAndTheOverlay drives the preparation of
// the M7 candidate through the simulated answers of the M3 one.
func TestM7PreparationInstallsTheToolsAndTheOverlay(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, version string
		fail          bool
	}{
		{"near the M7 candidate", m7CandidateVersion, false},
		{"red another version", m3CandidateVersion, true},
	} {
		s := m2TestSuite(t)
		s.m2.Ready = false
		s.execute = newM2Script(t, s, m3PrepareAnswer(t, s, tc.version)).Execute
		err := s.prepareM7(t.Context())
		if (err != nil) != tc.fail {
			t.Fatalf("%s: prepare = %v", tc.name, err)
		}
		if tc.fail {
			continue
		}
		proxy, err := m2FileURL(filepath.Join(s.base, m5ProxyDir))
		if err != nil {
			t.Fatal(err)
		}
		if !s.m2.Ready || environmentValue(s.env, "GOPROXY") != proxy+",https://proxy.golang.org,direct" ||
			environmentValue(s.env, m2DatabaseKey) == "" || len(s.digest) != 64 {
			t.Fatalf("%s: environment %q, digest %q", tc.name, s.env, s.digest)
		}
		scope := newM7Suite(s).m7Scope("m7-sample", "case")
		if scope.home == s.home || environmentValue(scope.env, "GOPROXY") != environmentValue(s.env, "GOPROXY") {
			t.Fatalf("%s: a case lost the overlay or shares the home", tc.name)
		}
	}
}

// TestM7GeneratorsBuildOnceIntoTheSharedDataHome: the generators build
// once per run into the data home every generator row shares, and a failed
// build is remembered with its cause instead of retried by every row.
func TestM7GeneratorsBuildOnceIntoTheSharedDataHome(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		code int
		fail bool
	}{
		{"near the generators built", 0, false},
		{"red a build refused", 2, true},
	} {
		s := newM7Suite(m2TestSuite(t))
		var data, capabilities []string
		s.execute = m5Script(t, s.suite,
			m5Reply{prefix: []string{commandInit}, once: true, answer: func(_ *testing.T, c command) result {
				capabilities = append(capabilities, c.args[2])
				return result{}
			}},
			m5Reply{program: s.goTool, prefix: []string{"mod", "tidy"}, answer: m5Answer(result{})},
			m5Reply{prefix: []string{m6SetupCommand, m7SetupGen}, once: true, answer: func(_ *testing.T, c command) result {
				data = append(data, environmentValue(c.env, envDataHome))
				return result{code: tc.code, stderr: fmt.Sprintf("golaw: exit %d\n", tc.code)}
			}},
		).Execute
		first, second := s.m7Generators(t.Context()), s.m7Generators(t.Context())
		if (first != nil) != tc.fail || (second != nil) != tc.fail || len(data) != 1 || data[0] != s.shared.data {
			t.Fatalf("%s: builds %q into %q, errors %v and %v", tc.name, data, s.shared.data, first, second)
		}
		if len(capabilities) != 1 || capabilities[0] != m7Capabilities {
			t.Fatalf("%s: the tools repository adopted %q", tc.name, capabilities)
		}
		if got := environmentValue(s.m7Scope("m7-sample", "gen").m7GenEnv(), envDataHome); got != s.shared.data {
			t.Fatalf("%s: a scope's generator environment names %q", tc.name, got)
		}
	}
}

// TestM7GeneratorsStopWhenTheToolsRepositoryIsNotAdopted: a refused git
// init or a refused golaw init stops the build before setup --gen runs, and
// the cause names the step that failed.
func TestM7GeneratorsStopWhenTheToolsRepositoryIsNotAdopted(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		git, adoption int
		want          string
	}{
		{"red git init refused", 2, 0, ""},
		{"red golaw init refused", 0, 2, "adopt the generator tools repository"},
	} {
		s := newM7Suite(m2TestSuite(t))
		built := false
		s.execute = m5Script(t, s.suite,
			m5Reply{program: s.git, prefix: []string{commandInit}, once: true, answer: m5Answer(result{code: tc.git})},
			m5Reply{prefix: []string{commandInit}, once: true, answer: m5Answer(result{code: tc.adoption})},
			m5Reply{prefix: []string{m6SetupCommand, m7SetupGen}, once: true, answer: func(_ *testing.T, _ command) result {
				built = true
				return result{}
			}},
		).Execute
		err := s.m7Generators(t.Context())
		if err == nil || built || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: error %v, setup ran %t", tc.name, err, built)
		}
	}
}

// TestM7GeneratorsStopBeforeSetupWhenTheMachineRefuses: a tools directory
// that cannot be written or a shell that cannot be found stops the build
// before setup --gen runs, so no row reads receipts that were never written.
func TestM7GeneratorsStopBeforeSetupWhenTheMachineRefuses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		blocked, shell bool
	}{
		{"red the tools directory blocked", true, false},
		{"red the shell missing", false, true},
	} {
		s := newM7Suite(m2TestSuite(t))
		if tc.blocked {
			if err := writeFile(filepath.Join(s.shared.base, m7ToolsRepo), []byte("file"), fileMode); err != nil {
				t.Fatal(err)
			}
		}
		if tc.shell {
			s.lookup = func(string) (string, error) { return "", ErrAcceptance }
		}
		built := false
		s.execute = m5Script(t, s.suite,
			m5Reply{prefix: []string{commandInit}, once: true, answer: m5Answer(result{})},
			m5Reply{prefix: []string{m6SetupCommand, m7SetupGen}, once: true, answer: func(_ *testing.T, _ command) result {
				built = true
				return result{}
			}},
		).Execute
		if err := s.m7Generators(t.Context()); err == nil || built {
			t.Fatalf("%s: error %v, setup ran %t", tc.name, err, built)
		}
	}
}

// TestM7GeneratorsNameARefusedAdoption: a tools repository golaw init
// refuses stops the build with the step that failed, before setup --gen.
func TestM7GeneratorsNameARefusedAdoption(t *testing.T) {
	t.Parallel()
	s := newM7Suite(m2TestSuite(t))
	s.execute = m5Script(t, s.suite,
		m5Reply{prefix: []string{commandInit}, once: true, answer: func(*testing.T, command) result {
			return result{code: 2, stderr: "golaw: exit 2\n"}
		}},
	).Execute
	if err := s.m7Generators(t.Context()); err == nil || !strings.Contains(err.Error(), "adopt the generator tools repository") {
		t.Fatalf("a refused adoption answered %v", err)
	}
}

// TestM7GeneratorsNameTheStepThatStoppedThem: a tools repository that
// cannot be written or adopted stops the build before setup --gen, and the
// cause every generator row reports names that step.
func TestM7GeneratorsNameTheStepThatStoppedThem(t *testing.T) {
	t.Parallel()
	t.Run("red the tools repository refused by init", func(t *testing.T) {
		t.Parallel()
		s := newM7Suite(m2TestSuite(t))
		s.execute = m5Script(t, s.suite,
			m5Reply{prefix: []string{commandInit}, once: true, answer: func(*testing.T, command) result {
				return result{code: 2, stderr: "golaw: exit 2\n"}
			}},
		).Execute
		if err := s.m7Generators(t.Context()); err == nil || !strings.Contains(err.Error(), "adopt the generator tools repository") {
			t.Fatalf("generators = %v, want the adoption named", err)
		}
	})
	t.Run("red the tools repository not writable", func(t *testing.T) {
		t.Parallel()
		s := newM7Suite(m2TestSuite(t))
		blocker := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(blocker, nil, fileMode); err != nil {
			t.Fatal(err)
		}
		s.shared.base = blocker
		s.execute = m5Script(t, s.suite).Execute
		if err := s.m7Generators(t.Context()); err == nil {
			t.Fatal("generators built under a regular file")
		}
	})
}
