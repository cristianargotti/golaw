package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestM2WindowsPathKeepsOnlyDeclaredSearch(t *testing.T) {
	t.Parallel()
	parent := []string{
		"Path=old-one", "PATH=old-two", "SystemRoot=system", "COMSPEC=command", "PATHEXT=.EXE;.CMD",
	}
	before := slices.Clone(parent)
	dirs := []string{"recorders", "candidate", "tools", "go", "git", "shell"}
	env := windowsPathM2(parent, dirs)
	if !slices.Equal(parent, before) {
		t.Fatal("Windows PATH replacement mutated its input")
	}
	want := []string{
		"SystemRoot=system", "COMSPEC=command", "PATHEXT=.EXE;.CMD",
		pathAssignment + strings.Join(dirs, string(os.PathListSeparator)),
	}
	if !slices.Equal(env, want) {
		t.Fatalf("Windows environment=%q want=%q", env, want)
	}
}

func TestM2WindowsRecorderEnvironmentAndProbe(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t).m2Scope("windows-recorders")
	s.m2OS = goosWindows
	script := m2NativeScript(t, s, "windows-shell-only", "answer")
	var calls []command
	s.execute = func(ctx context.Context, c command) result {
		calls = append(calls, c)
		return script.Execute(ctx, c)
	}
	proxy := filepath.Join(s.base, "scanner-proxy", executable("govulncheck"))
	m2Write(t, proxy, "scanner proxy\n")
	parent := prependPath(prependPath(s.env, s.m2.Bin), filepath.Dir(proxy))
	parent = append(parent, "Path=unwanted", "SystemRoot=system", "COMSPEC=command")
	before := slices.Clone(parent)
	repo := t.TempDir()
	env, err := s.m2WindowsEnvironment(t.Context(), repo, parent)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(parent, before) {
		t.Fatal("Windows recorder environment mutated its input")
	}
	recorders := environmentValue(env, "GOLAW_M2_RECORDERS")
	want := []string{recorders, filepath.Dir(s.binary), filepath.Dir(proxy), s.m2.Bin, filepath.Dir(s.goTool), filepath.Dir(s.git)}
	if got := filepath.SplitList(environmentValue(env, envPath)); !slices.Equal(got, want) {
		t.Fatalf("Windows directory order=%q want=%q", got, want)
	}
	for _, name := range []string{"make.exe", "awk.exe", "tar.exe"} {
		body, readErr := os.ReadFile(filepath.Join(recorders, name))
		if readErr != nil || string(body) != "simulated executable" {
			t.Fatalf("%s recorder bytes=%q error=%v", name, body, readErr)
		}
	}
	shell, err := s.lookup("sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.m2WindowsProbe(t.Context(), shell, repo, env); err != nil {
		t.Fatal(err)
	}
	m2AssertWindowsRecorderCommands(t, s, calls, shell, repo, recorders, env)
	if err := m2WindowsRecordersQuiet(env); err != nil {
		t.Fatal(err)
	}
	m2Write(t, filepath.Join(recorders, "make.started"), "started\n")
	if err := m2WindowsRecordersQuiet(env); err == nil {
		t.Fatal("recorder invocation without an exe suffix was ignored")
	}
}

func m2AssertWindowsRecorderCommands(t *testing.T, s *suite, calls []command, shell, repo, recorders string, env []string) {
	t.Helper()
	if len(calls) != 2 || calls[0].program != s.goTool || calls[0].args[0] != goBuild {
		t.Fatalf("recorder must compile once through the executor: %+v", calls)
	}
	probe := calls[1]
	if probe.program != shell || probe.dir != repo || !slices.Equal(probe.env, env) ||
		len(probe.args) != 4 || probe.args[0] != "-c" || probe.args[2] != "m2-probe" || probe.args[3] != recorders {
		t.Fatalf("same-shell recorder probe invocation differs: %+v", probe)
	}
	for _, fragment := range []string{
		`recorders=$(cd "$1" && pwd) || exit 90`,
		"for tool in make awk tar; do",
		`resolved=$(command -v "$tool") || exit 91`,
		`"$recorders/$tool"|"$recorders/$tool.exe") ;;`,
		`"recorder displaced: $tool: $resolved" >&2`,
		"exit 92",
	} {
		if !strings.Contains(probe.args[1], fragment) {
			t.Fatalf("probe lost %q", fragment)
		}
	}
}
