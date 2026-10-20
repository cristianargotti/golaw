package main

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestM2PrerequisiteShellRouting(t *testing.T) {
	t.Parallel()
	for _, system := range []string{"linux", goosWindows} {
		for _, includeTools := range []bool{false, true} {
			name := system + "-only"
			if includeTools {
				name = system + "-tools"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				s, shell := m2PrerequisiteSuite(t, system)
				bin := ""
				if includeTools {
					bin = s.m2.Bin
				}
				env, err := s.m2PrerequisiteEnvironment(s.env, bin)
				if err != nil {
					t.Fatal(err)
				}
				m2AssertPrerequisitePaths(t, s, shell, bin, env)
				m2AssertPrerequisitePrograms(t, s, shell, env)
			})
		}
	}
}

func m2PrerequisiteSuite(t *testing.T, system string) (*suite, string) {
	t.Helper()
	if runtime.GOOS == goosWindows && system != goosWindows {
		if err := os.Symlink("source", filepath.Join(t.TempDir(), "link")); err != nil {
			t.Skip("the non-Windows prerequisite branch requires symlink permission")
		}
	}
	s := m2TestSuite(t).m2Scope("prerequisites")
	s.m2Prerequisites = nil
	s.m2OS = system
	root := t.TempDir()
	s.goTool = filepath.Join(root, "go-bin", executable("go"))
	s.git = filepath.Join(root, "git-bin", executable(m2GitCommand))
	shell := filepath.Join(root, "shell-bin", executable("sh"))
	for _, path := range []string{s.goTool, s.git, shell} {
		m2Write(t, path, "simulated prerequisite\n")
	}
	s.lookup = func(name string) (string, error) {
		if name != "sh" {
			t.Fatalf("unexpected prerequisite lookup: %s", name)
		}
		return shell, nil
	}
	return s, shell
}

func m2AssertPrerequisitePaths(t *testing.T, s *suite, shell, bin string, env []string) {
	t.Helper()
	private := filepath.Join(s.base, "prerequisites")
	want := []string{private}
	if s.m2OS == goosWindows {
		want = []string{filepath.Dir(s.binary)}
		if bin != "" {
			want = append(want, bin)
		}
		want = append(want, filepath.Dir(s.goTool), filepath.Dir(s.git), filepath.Dir(shell))
		if _, err := os.Lstat(private); !os.IsNotExist(err) {
			t.Fatalf("Windows created private links: %v", err)
		}
	} else if bin != "" {
		want = append([]string{bin}, want...)
	}
	if got := filepath.SplitList(environmentValue(env, envPath)); !slices.Equal(got, want) {
		t.Fatalf("prerequisite PATH=%q want=%q", got, want)
	}
	if environmentValue(env, m2EnvGoBin) != bin {
		t.Fatalf("GOBIN differs from requested directory: %q", env)
	}
	if bin == "" && slices.ContainsFunc(env, func(entry string) bool { return strings.HasPrefix(entry, "GOBIN=") }) {
		t.Fatal("prerequisite-only environment retained GOBIN")
	}
}

func m2AssertPrerequisitePrograms(t *testing.T, s *suite, shell string, env []string) {
	t.Helper()
	for name, source := range map[string]string{
		"go": s.goTool, m2GitCommand: s.git, binaryName: s.binary, "sh": shell,
	} {
		path, err := resolveExecutable(name, env)
		if err != nil {
			t.Fatalf("%s prerequisite does not resolve: %v", name, err)
		}
		actual, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		expected, err := os.Stat(source)
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(actual, expected) {
			t.Fatalf("%s resolves to %q instead of %q", name, path, source)
		}
	}
}

func TestM2PrerequisiteSourcePollution(t *testing.T) {
	t.Parallel()
	for _, system := range []string{"linux", goosWindows} {
		t.Run(system, func(t *testing.T) {
			t.Parallel()
			s, _ := m2PrerequisiteSuite(t, system)
			m2Write(t, filepath.Join(filepath.Dir(s.git), executable("gitleaks")), "foreign tool")
			env, err := s.m2PrerequisiteEnvironment(s.env, "")
			if err != nil {
				t.Fatal(err)
			}
			got := m2MissingTools(env)
			if (got != nil) != (system == goosWindows) {
				t.Fatalf("pollution detection=%v system=%s", got, system)
			}
		})
	}
}

func TestM2PrerequisiteShellFailures(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"lookup", "missing", "nonexecutable"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			s, shell := m2PrerequisiteSuite(t, "linux")
			m2BreakPrerequisiteShell(t, s, shell, mode)
			if _, err := s.m2PrerequisiteEnvironment(s.env, ""); err == nil {
				t.Fatalf("accepted %s shell", mode)
			}
		})
	}
}

func m2BreakPrerequisiteShell(t *testing.T, s *suite, shell, mode string) {
	t.Helper()
	switch mode {
	case "lookup":
		s.lookup = func(string) (string, error) { return "", ErrAcceptance }
	case "missing":
		if err := os.Remove(shell); err != nil {
			t.Fatal(err)
		}
	case "nonexecutable":
		if err := os.Remove(shell); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(shell, dirMode); err != nil {
			t.Fatal(err)
		}
	}
}
