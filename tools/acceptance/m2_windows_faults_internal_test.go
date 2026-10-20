package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestM2WindowsRecorderPreparationFailures(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{
		"lookup", "shell-directory", "scanner", "source", "source-write",
		"recorder-directory", "build-error", "build-status", "build-output", "recorder-copy",
	} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			s := m2TestSuite(t).m2Scope(mode)
			s.m2OS = goosWindows
			s.execute = m2FixtureExecutor(t, s)
			env := m2PrepareWindowsFault(t, s, prependPath(s.env, s.m2.Bin), mode)
			if _, err := s.m2WindowsEnvironment(t.Context(), t.TempDir(), env); err == nil {
				t.Fatalf("Windows recorder preparation accepted %s", mode)
			}
		})
	}
}

func m2PrepareWindowsFault(t *testing.T, s *suite, env []string, mode string) []string {
	t.Helper()
	switch mode {
	case "lookup":
		s.lookup = func(string) (string, error) { return "", ErrAcceptance }
	case "shell-directory":
		dir := t.TempDir()
		s.lookup = func(string) (string, error) { return dir, nil }
	case "scanner":
		env = windowsPathM2(env, nil)
	case "source":
		s.root = t.TempDir()
	case "source-write":
		m2Write(t, s.base, "occupied")
	case "recorder-directory":
		m2Write(t, filepath.Join(s.base, "windows-recorders"), "occupied")
	case "build-error":
		s.execute = func(context.Context, command) result { return result{err: ErrAcceptance} }
	case "build-status":
		s.execute = func(context.Context, command) result { return result{code: 1} }
	case "build-output":
		s.execute = func(context.Context, command) result { return result{} }
	case "recorder-copy":
		fixture := s.execute
		s.execute = func(ctx context.Context, c command) result {
			got := fixture(ctx, c)
			if err := os.Mkdir(filepath.Join(s.base, "windows-recorders", "make.exe"), dirMode); err != nil {
				t.Fatal(err)
			}
			return got
		}
	}
	return env
}

func TestM2WindowsProbeRejectsDisplacementAndExecutionFailures(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"broken", "displaced-recorder", "noisy"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			s := m2TestSuite(t)
			s.execute = m2NativeScript(t, s, "windows-shell-only", mode).Execute
			if mode == "noisy" {
				s.execute = func(context.Context, command) result { return result{stdout: "unexpected\n"} }
			}
			env := []string{m2RecordersVariable + "=" + t.TempDir()}
			if err := s.m2WindowsProbe(t.Context(), "simulated-shell", t.TempDir(), env); err == nil {
				t.Fatalf("Windows probe accepted %s", mode)
			}
		})
	}
}

func TestM2WindowsRecorderObservationsRequireDirectory(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	s.execute = func(context.Context, command) result {
		t.Fatal("missing recorder environment started a process")
		return result{}
	}
	if err := s.m2WindowsProbe(t.Context(), "simulated-shell", t.TempDir(), nil); err == nil {
		t.Fatal("Windows probe accepted missing recorders")
	}
	if err := m2WindowsRecordersQuiet(nil); err == nil {
		t.Fatal("recorder check accepted missing environment")
	}
	env := []string{m2RecordersVariable + "=" + filepath.Join(t.TempDir(), "absent")}
	if err := m2WindowsRecordersQuiet(env); err == nil {
		t.Fatal("recorder check accepted an absent directory")
	}
}
