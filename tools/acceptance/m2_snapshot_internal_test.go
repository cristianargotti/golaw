package main

import (
	"bytes"
	"context"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestM2SnapshotPreservesIndexBytes(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	repo := m2SnapshotRepository(t, s)
	m2Write(t, filepath.Join(repo, "note.txt"), m2Changed)
	if err := s.gitSteps(t.Context(), repo, []string{gitAdd, "--", "note.txt"}); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(repo, ".git", "index")
	before, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	captured, err := s.captureM2(t.Context(), repo, s.env)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(index)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("snapshot changed index bytes: %v", err)
	}
	again, err := s.captureM2(t.Context(), repo, s.env)
	if err != nil || captured.Head == "" || captured.Tree == "" ||
		captured.Head != again.Head || captured.Tree != again.Tree ||
		!maps.Equal(captured.Files, again.Files) || !maps.Equal(captured.Indexes, again.Indexes) {
		t.Fatalf("snapshot is not stable: %+v %+v %v", captured, again, err)
	}
	if len(captured.Indexes) != 1 || len(captured.Files) != 1 {
		t.Fatalf("snapshot contains administrative files: %+v", captured)
	}
}

func TestM2SnapshotGitFailures(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"HEAD", "--absolute-git-dir", "write-tree"} {
		for _, mode := range []string{"process", "exit", "empty"} {
			s := m2TestSuite(t)
			repo := m2SnapshotRepository(t, s)
			s.execute = m2SnapshotGitFailure(t, s.execute, target, mode)
			if _, err := s.captureM2(t.Context(), repo, s.env); err == nil {
				t.Fatalf("%s %s: failed observation passed", target, mode)
			}
			copies, err := filepath.Glob(filepath.Join(s.base, "m2-index-*"))
			if err != nil || len(copies) != 0 {
				t.Fatalf("failed observation left copied indexes: %q %v", copies, err)
			}
		}
	}
}

func m2SnapshotGitFailure(t *testing.T, fixture executor, target, mode string) executor {
	t.Helper()
	return func(ctx context.Context, c command) result {
		if !slices.Contains(c.env, "GIT_OPTIONAL_LOCKS=0") {
			t.Fatal("snapshot did not disable optional locks")
		}
		if slices.Contains(c.args, target) {
			switch mode {
			case "process":
				return result{err: ErrAcceptance}
			case "exit":
				return result{code: 2, stderr: "failed"}
			case "empty":
				return result{}
			}
		}
		return fixture(ctx, c)
	}
}

func TestM2SnapshotTraversalFailures(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	missing := filepath.Join(root, "missing")
	if _, err := m2CheckoutDigests(missing, ""); err == nil {
		t.Fatal("missing checkout passed")
	}
	for _, mode := range []os.FileMode{0, os.ModeSymlink, os.ModeNamedPipe} {
		if _, err := m2CheckoutBytes(missing, mode); err == nil {
			t.Fatalf("unreadable checkout entry %v passed", mode)
		}
	}
	s := m2TestSuite(t)
	repo := m2SnapshotRepository(t, s)
	env := append(slices.Clone(s.env), "GIT_INDEX_FILE=missing")
	if _, err := s.captureM2(t.Context(), repo, env); err == nil {
		t.Fatal("missing alternate index passed")
	}
	s.base = filepath.Join(repo, "note.txt")
	if _, err := s.captureM2(t.Context(), repo, s.env); err == nil {
		t.Fatal("invalid temporary directory passed")
	}
	if _, err := s.m2SnapshotTreeAt(t.Context(), repo, s.env, nil, s.base); err == nil {
		t.Fatal("failed temporary index write passed")
	}
}

func TestM2SnapshotFilesystemFaults(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission faults require mode enforcement")
	}
	for _, fault := range []string{"checkout", "cleanup"} {
		s := m2TestSuite(t)
		repo := m2SnapshotRepository(t, s)
		s.execute = m2SnapshotFilesystemFailure(t, s.execute, repo, fault)
		if _, err := s.captureM2(t.Context(), repo, s.env); err == nil {
			t.Fatalf("%s: filesystem failure passed", fault)
		}
	}
}

func m2SnapshotFilesystemFailure(t *testing.T, fixture executor, repo, fault string) executor {
	t.Helper()
	return func(ctx context.Context, c command) result {
		got := fixture(ctx, c)
		if !slices.Contains(c.args, "write-tree") {
			return got
		}
		path := filepath.Join(repo, "note.txt")
		if fault == "cleanup" {
			path = filepath.Dir(environmentValue(c.env, "GIT_INDEX_FILE"))
		}
		t.Cleanup(func() {
			if err := os.Chmod(path, dirMode); err != nil {
				t.Error(err)
			}
		})
		if err := os.Chmod(path, 0); err != nil {
			t.Fatal(err)
		}
		return got
	}
}

func TestM2SnapshotSelectedIndexAndLinkedWorktree(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	repo := m2SnapshotRepository(t, s)
	linked := filepath.Join(s.base, "linked")
	if err := s.gitSteps(t.Context(), repo, []string{"worktree", "add", "-q", "-b", "snapshot", linked}); err != nil {
		t.Fatal(err)
	}
	gitDir := filepath.Join(repo, ".git", "worktrees", filepath.Base(linked))
	relativeGit, err := filepath.Rel(linked, gitDir)
	if err != nil {
		t.Fatal(err)
	}
	common, err := filepath.Rel(linked, filepath.Join(repo, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	env := append(slices.Clone(s.env), "GIT_DIR="+relativeGit, "GIT_COMMON_DIR="+common,
		"GIT_INDEX_FILE="+filepath.Join(relativeGit, "alternate"), "GIT_PREFIX=")
	s.env = env
	if stepErr := s.gitSteps(t.Context(), linked, []string{"read-tree", "HEAD"}); stepErr != nil {
		t.Fatal(stepErr)
	}
	m2Write(t, filepath.Join(linked, "note.txt"), m2Changed)
	if stepErr := s.gitSteps(t.Context(), linked, []string{gitAdd, "--", "note.txt"}); stepErr != nil {
		t.Fatal(stepErr)
	}
	before, err := treeDigests(gitDir)
	if err != nil {
		t.Fatal(err)
	}
	s.execute = m2SnapshotEnvironment(t, s.execute, env)
	got, err := s.captureM2(t.Context(), linked, env)
	if err != nil {
		t.Fatal(err)
	}
	after, err := treeDigests(gitDir)
	if err != nil || !maps.Equal(before, after) {
		t.Fatalf("snapshot changed linked administrative bytes: %v", err)
	}
	if len(got.Indexes) != 2 || got.Indexes[filepath.Join(gitDir, "index")] ==
		got.Indexes[filepath.Join(gitDir, "alternate")] || len(got.Files) != 1 {
		t.Fatalf("alternate index or linked administrative file lost: %+v", got)
	}
	if _, found := got.Files[filepath.Join(linked, ".git")]; found {
		t.Fatal("linked administrative file entered checkout snapshot")
	}
}

func m2SnapshotEnvironment(t *testing.T, fixture executor, env []string) executor {
	t.Helper()
	return func(ctx context.Context, c command) result {
		for _, key := range []string{"GIT_DIR", "GIT_COMMON_DIR", "GIT_PREFIX"} {
			if environmentValue(c.env, key) != environmentValue(env, key) {
				t.Fatalf("%s changed in observation", key)
			}
		}
		return fixture(ctx, c)
	}
}

func m2SnapshotRepository(t *testing.T, s *suite) string {
	t.Helper()
	s.execute = m2FixtureExecutor(t, s)
	repo, err := s.repository(t.Context(), "snapshot", false)
	if err != nil {
		t.Fatal(err)
	}
	m2Write(t, filepath.Join(repo, "note.txt"), m2Safe)
	if err := s.gitSteps(t.Context(), repo, []string{commandInit, "-q"},
		[]string{gitAdd, "--", "note.txt"}, commitArgs("test: snapshot baseline")); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestM2SnapshotRejectsFilesystemFailures(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"missing", "directory"} {
		s := m2TestSuite(t)
		repo := m2SnapshotRepository(t, s)
		index := filepath.Join(repo, ".git", "index")
		if err := os.Remove(index); err != nil {
			t.Fatal(err)
		}
		if mode == "directory" {
			if err := os.Mkdir(index, dirMode); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := s.captureM2(t.Context(), repo, s.env); err == nil {
			t.Fatalf("%s: unreadable index passed", mode)
		}
	}
}

func TestM2SnapshotIgnoresOutsideSymlinkContents(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	repo := m2SnapshotRepository(t, s)
	outside := filepath.Join(t.TempDir(), "outside")
	m2Write(t, outside, m2Safe)
	if err := os.Symlink(outside, filepath.Join(repo, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	before, err := s.captureM2(t.Context(), repo, s.env)
	if err != nil {
		t.Fatal(err)
	}
	m2Write(t, outside, m2Changed)
	after, err := s.captureM2(t.Context(), repo, s.env)
	if err != nil || !maps.Equal(before.Files, after.Files) {
		t.Fatalf("snapshot followed an outside link: %v", err)
	}
	if _, found := before.Files[filepath.Join(repo, "link")]; !found {
		t.Fatal("symlink target was not captured")
	}
	for path := range after.Files {
		if !strings.HasPrefix(path, repo+string(filepath.Separator)) {
			t.Fatalf("snapshot escaped checkout: %s", path)
		}
	}
}
