package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestM2TimingRejectsIncompleteInputs(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"rung", "cache", "shared", "unverified"} {
		s := m2TestSuite(t)
		name, cache := "pre-commit", t.TempDir()
		switch mode {
		case "rung":
			name = "unknown"
		case "cache":
			cache = ""
		case "shared":
			s.m2 = nil
		case "unverified":
			s.m2.Ready = false
		}
		s.execute = func(context.Context, command) result {
			t.Fatal("incomplete timing inputs started preparation")
			return result{}
		}
		if _, err := s.timingM2(t.Context(), name, cache); err == nil {
			t.Fatalf("%s: incomplete timing inputs passed", mode)
		}
	}
}

func TestM2TimingPreparationPropagatesCommandFailures(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"source", "git-init", "init", "baseline", "stage", "push-commit", "proxy", "head"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			s := m2TestSuite(t)
			script := m2TimingScript(t, s, "answer")
			reached := false
			s.execute = func(ctx context.Context, c command) result {
				if m2TimingCommandFailure(s, c, mode) {
					reached = true
					return result{err: ErrAcceptance}
				}
				return script.Execute(ctx, c)
			}
			if _, err := s.timingM2(t.Context(), "pre-push", t.TempDir()); err == nil || !reached {
				t.Fatalf("%s: reached=%v error=%v", mode, reached, err)
			}
		})
	}
}

func m2TimingCommandFailure(s *suite, c command, mode string) bool {
	switch mode {
	case "source":
		return c.program == s.git && slices.Contains(c.args, "ls-files")
	case "git-init":
		return c.program == s.git && m2Subcommand(c.args) == commandInit
	case "init":
		return c.program == s.binary && slices.Equal(c.args, []string{commandInit, flagApply})
	case "baseline":
		return c.program == s.git && slices.Contains(c.args, "test: prepare lifecycle fixture")
	case "stage":
		return c.program == s.git && slices.Equal(c.args, []string{gitAdd, "--", m2TimingSourcePath})
	case "push-commit":
		return c.program == s.git && slices.Contains(c.args, "test: prepare measured push")
	case "proxy":
		return c.program == s.goTool && len(c.args) > 0 && c.args[0] == goBuild
	case "head":
		return c.program == s.git && slices.Equal(c.args, []string{"-c", "core.quotePath=false", m2RevParse, m2Verify, m2Head})
	}
	return false
}

func TestM2TimingPreparationPropagatesFileFailures(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"repository", "source", "doc", "policy", "hooks"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			s := m2TestSuite(t)
			script := m2TimingScript(t, s, "answer")
			s.execute = script.Execute
			m2FailTimingFile(t, s, script, mode)
			if _, err := s.m2TimingRepository(t.Context(), "pre-commit"); err == nil {
				t.Fatalf("%s: failed file preparation passed", mode)
			}
		})
	}
}

func m2FailTimingFile(t *testing.T, s *suite, script *m2Script, mode string) {
	t.Helper()
	switch mode {
	case "repository":
		m2Write(t, filepath.Join(s.base, dirRepos), "occupied")
	case "source":
		s.root = t.TempDir()
	case "doc":
		s.execute = func(ctx context.Context, c command) result {
			got := script.Execute(ctx, c)
			if slices.Contains(c.args, "test: prepare lifecycle fixture") {
				if err := os.Remove(filepath.Join(c.dir, filepath.FromSlash(m2TimingSourcePath))); err != nil {
					t.Fatal(err)
				}
			}
			return got
		}
	case "policy", "hooks":
		name := policyFile
		if mode == "hooks" {
			name = "lefthook.yml/occupied"
		}
		m2Write(t, filepath.Join(s.base, dirRepos, "kit", filepath.FromSlash(name)), "invalid")
	}
}

func TestM2TimingCopyAndArgumentFailures(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	if err := s.m2CopyTimingSource(t.TempDir(), []m2ArchiveFile{{name: "missing"}}); err == nil {
		t.Fatal("missing source mode passed")
	}
	blocked := filepath.Join(t.TempDir(), "blocked")
	m2Write(t, blocked, "occupied")
	if err := s.m2CopyTimingSource(blocked, []m2ArchiveFile{{name: moduleFile}}); err == nil {
		t.Fatal("blocked source copy passed")
	}
	if err := os.MkdirAll(filepath.Join(s.base, m2MessageFile), dirMode); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.m2TimingArguments(t.Context(), s.base, "commit-msg", s.env); err == nil {
		t.Fatal("unwritable message passed")
	}
}

func TestM2TimingLivePushPreparationFailures(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"shell", "dispatcher", "scanner", "write"} {
		s := m2TestSuite(t)
		s.m2.Live = true
		switch mode {
		case "shell":
			s.lookup = func(string) (string, error) { return "", ErrAcceptance }
		case "dispatcher", "scanner":
			name := "lefthook"
			if mode == "scanner" {
				name = m2Scanner
			}
			if err := os.Remove(filepath.Join(s.m2.Bin, executable(name))); err != nil {
				t.Fatal(err)
			}
		case "write":
			if err := os.MkdirAll(filepath.Join(s.base, m2PushToolsDir, executable(m2Scanner)), dirMode); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := s.m2TimingPushEnvironment(t.Context(), s.base, s.env); err == nil {
			t.Fatalf("%s: failed live push preparation passed", mode)
		}
	}
}
