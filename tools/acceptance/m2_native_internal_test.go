package main

import (
	"slices"
	"testing"
)

func TestM2InstalledRungs(t *testing.T) {
	t.Parallel()
	names := []string{
		"installed-pre-commit-red",
		"installed-pre-commit-near",
		"installed-commit-msg-red",
		"installed-commit-msg-near",
		"installed-pre-push-red",
		"installed-pre-push-near",
		"spawn-git-red",
		"spawn-git-near",
		"windows-shell-only",
	}
	for _, goos := range []string{"linux", "darwin", "windows"} {
		for _, name := range names {
			m2NativeModes(t, goos, name)
		}
	}
	if !slices.Equal(m2NativeNames(), names) {
		t.Fatalf("native catalog = %q", m2NativeNames())
	}
}

// m2NativeModes runs one native row in its answer, corrupt and broken
// modes on one operating system; the Windows-only row is an optional SKIP
// elsewhere.
func m2NativeModes(t *testing.T, goos, name string) {
	t.Helper()
	for _, mode := range []struct {
		name string
		want string
	}{
		{"answer", statusPass},
		{"corrupt", statusFail},
		{"broken", statusFail},
	} {
		s := m2TestSuite(t).m2Scope(name + "-" + mode.name)
		s.m2OS = goos
		script := m2NativeScript(t, s, name, mode.name)
		s.execute = script.Execute
		got := s.nativeM2(t.Context(), name)
		if name == "windows-shell-only" && goos != "windows" {
			if got.status != statusSkip || got.required || got.detail == "" {
				t.Fatalf("native-only result = %+v", got)
			}
			continue
		}
		if got.status != mode.want || !got.required {
			t.Fatalf("%s %s %s: %+v", goos, name, mode.name, got)
		}
		if script.Calls == 0 {
			t.Fatalf("%s: executor was not used", name)
		}
	}
}

func TestM2NativeRowsRejectExecutionFailures(t *testing.T) {
	t.Parallel()
	for _, name := range m2NativeNames() {
		s := m2TestSuite(t)
		s.m2OS = "windows"
		script := m2NativeScript(t, s, name, "broken")
		s.execute = script.Execute
		got := s.nativeM2(t.Context(), name)
		if got.status != statusFail || !got.required {
			t.Fatalf("%s accepted execution failure: %+v", name, got)
		}
	}
}

func TestM2WindowsPathReplacesInheritedSearch(t *testing.T) {
	t.Parallel()
	env := windowsPathM2(
		[]string{"Path=unwanted", "SystemRoot=system", "COMSPEC=command"},
		[]string{"recorders", "candidate", "tools", "go", "git", "shell"},
	)
	path := environmentValue(env, "PATH")
	if path == "unwanted" || path == "" {
		t.Fatal("inherited PATH survived")
	}
	if environmentValue(env, "SystemRoot") != "system" ||
		environmentValue(env, "COMSPEC") != "command" {
		t.Fatal("Windows process prerequisites were lost")
	}
}

func TestM2WindowsRecorderControls(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"displaced-recorder", "recorder-started"} {
		s := m2TestSuite(t)
		s.m2OS = "windows"
		script := m2NativeScript(t, s, "windows-shell-only", mode)
		s.execute = script.Execute
		got := s.nativeM2(t.Context(), "windows-shell-only")
		if got.status != statusFail || !got.required {
			t.Fatalf("%s passed: %+v", mode, got)
		}
	}
}
