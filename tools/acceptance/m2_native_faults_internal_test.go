package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestM2NativeRowsRejectPreparationFailures(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"unknown", "repository", "sample", "init", "setup", "push-build", "windows-build", "windows-tests"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			s := m2TestSuite(t)
			s.m2OS = goosWindows
			name := "installed-pre-commit-near"
			switch mode {
			case "unknown":
				name = mode
			case "push-build":
				name = "installed-pre-push-near"
			case "windows-build", "windows-tests":
				name = m2WindowsOnly
			}
			script := m2NativeScript(t, s, name, "answer")
			s.execute = m2NativePreparationFault(t, s, script.Execute, mode)
			if mode == "repository" {
				m2Write(t, filepath.Join(s.base, dirRepos), "occupied")
			}
			if mode == "sample" {
				m2Write(t, filepath.Join(s.base, dirRepos, name, "cmd"), "occupied")
			}
			got := s.nativeM2(t.Context(), name)
			if got.status != statusFail || !got.required || script.Targets != 0 {
				t.Fatalf("mode=%s targets=%d result=%+v", mode, script.Targets, got)
			}
		})
	}
}

func m2NativePreparationFault(t *testing.T, s *suite, fixture executor, mode string) executor {
	t.Helper()
	return func(ctx context.Context, c command) result {
		switch {
		case mode == "init" && m2Kind(s, c) == "init",
			mode == "setup" && m2Kind(s, c) == m2SetupCommand,
			(mode == "push-build" || mode == "windows-build") && c.program == s.goTool:
			return result{err: ErrAcceptance}
		}
		got := fixture(ctx, c)
		if mode == "windows-tests" && m2Kind(s, c) == "doctor" {
			body, err := os.ReadFile(filepath.Join(c.dir, policyFile))
			if err != nil {
				t.Fatal(err)
			}
			m2Write(t, filepath.Join(c.dir, policyFile), string(body)+"invalid: true\n")
		}
		return got
	}
}

func TestM2NativeStagingRejectsUnavailableState(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"write", "add", "checkout", "snapshot"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			s, repo := m2PreparedNative(t)
			if mode == "write" {
				m2NativeBlockNote(t, repo)
			}
			s.execute = m2NativeStageFault(t, s, s.execute, repo, mode)
			if err := s.m2NativeStaged(t.Context(), repo, false); err == nil {
				t.Fatalf("accepted %s failure", mode)
			}
		})
	}
}

func m2NativeStageFault(t *testing.T, s *suite, fixture executor, repo, mode string) executor {
	t.Helper()
	return func(ctx context.Context, c command) result {
		stage := c.program == s.git && slices.Equal(c.args, []string{gitAdd, "--", pathNote})
		if stage && mode == "add" {
			return result{err: ErrAcceptance}
		}
		got := fixture(ctx, c)
		if stage && mode == "checkout" {
			m2NativeBlockNote(t, repo)
		}
		if stage && mode == "snapshot" {
			if err := os.Remove(filepath.Join(repo, gitDirectory, "index")); err != nil {
				t.Fatal(err)
			}
		}
		return got
	}
}

func m2NativeBlockNote(t *testing.T, repo string) {
	t.Helper()
	path := filepath.Join(repo, pathNote)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, dirMode); err != nil {
		t.Fatal(err)
	}
}

func TestM2NativeMessageAndPushRequireInputs(t *testing.T) {
	t.Parallel()
	s, repo := m2PreparedNative(t)
	if err := os.Mkdir(filepath.Join(s.base, m2MessageFile), dirMode); err != nil {
		t.Fatal(err)
	}
	if err := s.m2NativeMessage(t.Context(), repo, false, s.env); err == nil {
		t.Fatal("missing message passed")
	}
	s.execute = func(context.Context, command) result { return result{err: ErrAcceptance} }
	if err := s.m2NativePush(t.Context(), repo, false, s.env); err == nil {
		t.Fatal("missing push commit passed")
	}
	if _, err := s.installedM2(t.Context(), repo, "pre-commit", "", nil, nil); err == nil {
		t.Fatal("missing state environment passed")
	}
}

func TestM2NativeWindowsRequiresTestsDisabled(t *testing.T) {
	t.Parallel()
	for _, body := range []string{"", "[", "golaw: 1\nkit: {version: 0.2.0}\n", "golaw: 1\nkit: {version: 0.2.0}\nladder: {test_staged: true}\n"} {
		repo := t.TempDir()
		if body != "" {
			m2Write(t, filepath.Join(repo, policyFile), body)
		}
		if err := m2NativeTestsDisabled(repo); err == nil {
			t.Fatalf("accepted policy %q", body)
		}
	}
}
