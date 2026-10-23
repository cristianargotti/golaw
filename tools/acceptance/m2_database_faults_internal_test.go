package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestM2DatabaseRejectsPreparationAndBuildFailures(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{
		"unverified", "missing-state", "scanner", "scanner-directory", "database", "source-directory",
		"execution", "exit", "missing-output",
	} {
		s := m2TestSuite(t)
		m2BreakDatabasePreparation(t, s, mode)
		fixture := m2FixtureExecutor(t, s)
		s.execute = func(ctx context.Context, c command) result {
			switch mode {
			case "execution":
				return result{err: ErrAcceptance}
			case "exit":
				return result{code: 2}
			case "missing-output":
				return result{}
			}
			return fixture(ctx, c)
		}
		if _, err := s.m2DatabaseEnvironment(t.Context(), t.TempDir(), s.env, false); err == nil {
			t.Fatalf("%s: invalid scanner preparation passed", mode)
		}
	}
}

func m2BreakDatabasePreparation(t *testing.T, s *suite, mode string) {
	t.Helper()
	switch mode {
	case "unverified":
		s.m2.Ready = false
	case "missing-state":
		s.m2 = nil
	case "scanner", "scanner-directory":
		path := filepath.Join(s.m2.Bin, executable(m2Scanner))
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if mode == "scanner-directory" {
			if err := os.Mkdir(path, dirMode); err != nil {
				t.Fatal(err)
			}
		}
	case "database":
		s.root = t.TempDir()
	case "source-directory":
		m2Write(t, filepath.Join(s.base, "scanner-source"), "not a directory")
	}
}

func TestM2DatabaseRequiresProxySource(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	s.root = t.TempDir()
	s.execute = func(context.Context, command) result {
		t.Fatal("build started without the supplied proxy source")
		return result{}
	}
	if err := s.m2BuildScannerProxy(t.Context(), t.TempDir(), filepath.Join(s.base, executable(m2Scanner)), s.env); err == nil {
		t.Fatal("absent proxy source passed")
	}
}

func TestM2PushRejectsInvalidDispatcherAndIsolation(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{
		"unverified", "missing-state", "shell", "dispatcher", "dispatcher-directory", "private-directory", "pollution",
	} {
		s := m2TestSuite(t)
		m2BreakPushPreparation(t, s, mode)
		s.execute = m2FixtureExecutor(t, s)
		if _, err := s.m2PushEnvironment(t.Context(), t.TempDir(), s.env, false); err == nil {
			t.Fatalf("%s: invalid push isolation passed", mode)
		}
	}
}

func m2BreakPushPreparation(t *testing.T, s *suite, mode string) {
	t.Helper()
	switch mode {
	case "unverified", "missing-state":
		m2BreakDatabasePreparation(t, s, mode)
	case "shell":
		s.lookup = func(string) (string, error) { return "", ErrAcceptance }
	case "dispatcher", "dispatcher-directory":
		path := filepath.Join(s.m2.Bin, executable("lefthook"))
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if mode == "dispatcher-directory" {
			if err := os.Mkdir(path, dirMode); err != nil {
				t.Fatal(err)
			}
		}
	case "private-directory":
		m2Write(t, filepath.Join(s.base, m2PushToolsDir), "not a directory")
	case "pollution":
		m2Write(t, filepath.Join(s.base, m2PushToolsDir, executable("golangci-lint")), "unrelated tool")
	}
}

func TestM2PushReportsUnreadableSearchDirectories(t *testing.T) {
	t.Parallel()
	if err := m2PushToolsAbsent([]string{pathAssignment + m2UnreadableParent(t)}); err == nil {
		t.Fatal("invalid search directory passed")
	}
}
