package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// m3FailingAt answers the nth command a script runs with a process that
// could not start and leaves every other command to the answer, so a test
// stops a sequence at each of its steps in turn.
func m3FailingAt(n int, answer m2Answer) m2Answer {
	calls := 0
	return func(ctx context.Context, c command) (m2Reply, bool) {
		calls++
		if calls == n {
			return m2Reply{got: result{err: ErrAcceptance}}, true
		}
		return answer(ctx, c)
	}
}

func TestM3PreparationStopsAtEveryFailedStep(t *testing.T) {
	t.Parallel()
	prepare := func(t *testing.T, n int) (*m2Script, error) {
		t.Helper()
		s := m2TestSuite(t)
		s.m2.Ready = false
		script := newM2Script(t, s, m3FailingAt(n, m3PrepareAnswer(t, s, m3CandidateVersion)))
		s.execute = script.Execute
		return script, s.prepareM3(t.Context())
	}
	script, err := prepare(t, 0)
	if err != nil || script.Calls == 0 {
		t.Fatalf("near the whole preparation = %v after %d commands", err, script.Calls)
	}
	for n := 1; n <= script.Calls; n++ {
		t.Run(fmt.Sprintf("red step %d", n), func(t *testing.T) {
			t.Parallel()
			if _, err := prepare(t, n); !errors.Is(err, ErrAcceptance) {
				t.Fatalf("step %d of %d = %v", n, script.Calls, err)
			}
		})
	}
}

func TestM3PreparationNeedsMake(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	s.m2.Ready = false
	s.execute = newM2Script(t, s, m3PrepareAnswer(t, s, m3CandidateVersion)).Execute
	lookup := s.lookup
	s.lookup = func(name string) (string, error) {
		if name == m3MakeName {
			return "", ErrAcceptance
		}
		return lookup(name)
	}
	if err := s.prepareM3(t.Context()); !errors.Is(err, ErrAcceptance) || !strings.Contains(err.Error(), "locate make") {
		t.Fatalf("a machine without make = %v", err)
	}
}

func TestM3ServiceRepositoryStopsAtEveryFailedStep(t *testing.T) {
	t.Parallel()
	adopt := func(t *testing.T, n int) (*m2Script, error) {
		t.Helper()
		s := m2TestSuite(t)
		var calls [][]string
		script := newM2Script(t, s, m3FailingAt(n, m3ServiceAnswer(t, s, &calls)))
		s.execute = script.Execute
		_, err := s.m3ServiceRepository(t.Context(), "adopted", []string{"go"})
		return script, err
	}
	script, err := adopt(t, 0)
	if err != nil || script.Calls == 0 {
		t.Fatalf("near the whole adoption = %v after %d commands", err, script.Calls)
	}
	for n := 1; n <= script.Calls; n++ {
		t.Run(fmt.Sprintf("red step %d", n), func(t *testing.T) {
			t.Parallel()
			if _, err := adopt(t, n); !errors.Is(err, ErrAcceptance) {
				t.Fatalf("step %d of %d = %v", n, script.Calls, err)
			}
		})
	}
}

func TestM3RepositoriesNeedTheirDirectory(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	s.execute = func(context.Context, command) result { return result{err: ErrAcceptance} }
	checkedWrite(t, filepath.Join(s.base, dirRepos), "a file where the repositories go\n")
	if _, err := s.m3ServiceRepository(t.Context(), "blocked", []string{"go"}); err == nil {
		t.Fatal("a service repository over a file must fail")
	}
	if _, err := s.m3ToolsRepository(t.Context()); err == nil {
		t.Fatal("a tools repository over a file must fail")
	}
	blocked := filepath.Join(t.TempDir(), "file")
	checkedWrite(t, blocked, "a file where the service goes\n")
	if err := m3WriteService(blocked); err == nil {
		t.Fatal("the service written under a file must fail")
	}
}

func TestM3AcceptModuleFollowsTheRulesFile(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	started := false
	s.execute = func(context.Context, command) result {
		started = true
		return result{err: ErrAcceptance}
	}
	repo := t.TempDir()
	if err := s.m3AcceptModule(t.Context(), repo); err != nil || started {
		t.Fatalf("near no rules file = %v, started %v", err, started)
	}
	checkedWrite(t, filepath.Join(repo, filepath.FromSlash(m3RulesFile)), "package rules\n")
	s.root = t.TempDir()
	if err := s.m3AcceptModule(t.Context(), repo); err == nil || started {
		t.Fatalf("red a kit without go.mod = %v, started %v", err, started)
	}
}

func TestM3CopyTreeNamesWhatItCannotCopy(t *testing.T) {
	t.Parallel()
	if err := m3CopyTree(filepath.Join(t.TempDir(), "absent"), filepath.Join(t.TempDir(), "copy")); err == nil {
		t.Fatal("an absent source must fail")
	}
	source := t.TempDir()
	checkedWrite(t, filepath.Join(source, "dir", "a.txt"), "a\n")
	target := filepath.Join(t.TempDir(), "file")
	checkedWrite(t, target, "a file where the copy goes\n")
	if err := m3CopyTree(source, target); err == nil {
		t.Fatal("a copy over a file must fail")
	}
}

func TestM3CopyTreeSkipsAnEntryThatVanishes(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	lock := filepath.Join(source, "maintenance.lock")
	checkedWrite(t, lock, "held\n")
	entries, err := os.ReadDir(source)
	if err != nil || len(entries) != 1 {
		t.Fatalf("list the source = %v, %v", entries, err)
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	if err := m3CopyFile(lock, filepath.Join(target, "maintenance.lock"), entries[0]); err != nil {
		t.Fatalf("a file gone after the listing = %v, want it skipped", err)
	}
	gone := fmt.Errorf("read the directory: %w", fs.ErrNotExist)
	if err := m3CopyEntry(source, target, filepath.Join(source, "objects"), nil, gone); err != nil {
		t.Fatalf("a directory gone after the listing = %v, want it skipped", err)
	}
	if err := m3CopyEntry(source, target, source, nil, gone); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("an absent root = %v, want it named", err)
	}
	if err := m3CopyEntry(source, target, filepath.Join(source, "denied"), nil, fs.ErrPermission); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("an unreadable entry = %v, want it named", err)
	}
}
