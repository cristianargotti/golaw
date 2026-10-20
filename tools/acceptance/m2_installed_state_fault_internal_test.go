package main

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestM2InstalledObservationPreservesBothRawIndexes(t *testing.T) {
	t.Parallel()
	s, repo, paths := m2InstalledObservationFixture(t)
	original := slices.Clone(s.env)
	before := m2InstalledIndexBytes(t, paths)
	fixture := s.execute
	var copies []string
	s.execute = func(ctx context.Context, c command) result {
		if !slices.Contains(c.args, "ls-files") {
			return fixture(ctx, c)
		}
		copied := m2RequireCopiedIndexListing(t, s, repo, original, before, c)
		copies = append(copies, copied)
		got := fixture(ctx, c)
		m2Write(t, copied, "copied administrative bytes changed\n")
		return got
	}
	got, err := s.captureM2Installed(t.Context(), repo, s.env)
	if err != nil {
		t.Fatal(err)
	}
	if len(copies) != 2 || len(got.Indexes) != 2 || got.Indexes[paths[0]] == got.Indexes[paths[1]] {
		t.Fatalf("both indexes were not observed independently: copies=%q state=%+v", copies, got)
	}
	if !maps.Equal(before, m2InstalledIndexBytes(t, paths)) || !slices.Equal(original, s.env) {
		t.Fatal("installed snapshot changed original indexes or caller environment")
	}
	for _, path := range copies {
		if _, err := os.Lstat(filepath.Dir(path)); !os.IsNotExist(err) {
			t.Fatalf("copied index directory survived: %s: %v", path, err)
		}
	}
}

func m2InstalledObservationFixture(t *testing.T) (*suite, string, []string) {
	t.Helper()
	s := m2TestSuite(t)
	repo := m2SnapshotRepository(t, s)
	gitDir := filepath.Join(repo, gitDirectory)
	paths := []string{filepath.Join(gitDir, "index"), filepath.Join(gitDir, "alternate")}
	body, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	m2Write(t, paths[1], string(body))
	s.env = append(s.env, "GIT_DIR=.git", "GIT_COMMON_DIR=.git", "GIT_PREFIX=",
		"GIT_INDEX_FILE="+filepath.Join(gitDirectory, "alternate"), "git_optional_locks=1")
	m2Write(t, filepath.Join(repo, pathNote), m2Changed)
	if err := s.gitSteps(t.Context(), repo, []string{gitAdd, "--", pathNote}); err != nil {
		t.Fatal(err)
	}
	return s, repo, paths
}

func m2InstalledIndexBytes(t *testing.T, paths []string) map[string]string {
	t.Helper()
	out := make(map[string]string, len(paths))
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		out[path] = string(body)
	}
	return out
}

func m2RequireCopiedIndexListing(t *testing.T, s *suite, repo string, env []string, originals map[string]string, c command) string {
	t.Helper()
	if c.program != s.git || c.dir != repo || !slices.Equal(c.args, []string{"ls-files", "--stage", "-z"}) {
		t.Fatalf("unexpected index observation: %+v", c)
	}
	for _, key := range []string{"GIT_DIR", "GIT_COMMON_DIR", "GIT_PREFIX"} {
		if environmentValue(c.env, key) != environmentValue(env, key) {
			t.Fatalf("index observation changed %s", key)
		}
	}
	path := environmentValue(c.env, "GIT_INDEX_FILE")
	if _, found := originals[path]; found || !strings.HasPrefix(path, s.base+string(filepath.Separator)) {
		t.Fatalf("index observation did not use a private copy: %s", path)
	}
	if environmentValue(c.env, "GIT_OPTIONAL_LOCKS") != "0" {
		t.Fatal("index observation retained optional locks")
	}
	m2RequireUniqueIndexVariables(t, c.env)
	body, err := os.ReadFile(path)
	if err != nil || !slices.Contains(slices.Collect(maps.Values(originals)), string(body)) {
		t.Fatalf("index copy differs from both original indexes: %v", err)
	}
	return path
}

func m2RequireUniqueIndexVariables(t *testing.T, env []string) {
	t.Helper()
	for _, key := range []string{"GIT_INDEX_FILE", "GIT_OPTIONAL_LOCKS"} {
		count := 0
		for _, entry := range env {
			name, _, _ := strings.Cut(entry, "=")
			if strings.EqualFold(name, key) {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("index observation has %d entries for %s", count, key)
		}
	}
}

func TestM2InstalledIndexListingFailuresCleanCopies(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"execution", "exit"} {
		s, repo, paths := m2InstalledObservationFixture(t)
		before := m2InstalledIndexBytes(t, paths)
		fixture := s.execute
		var copied string
		s.execute = func(ctx context.Context, c command) result {
			if !slices.Contains(c.args, "ls-files") {
				return fixture(ctx, c)
			}
			copied = m2RequireCopiedIndexListing(t, s, repo, s.env, before, c)
			if mode == "execution" {
				return result{err: ErrAcceptance}
			}
			return result{code: 2, stderr: "listing failed"}
		}
		if _, err := s.captureM2Installed(t.Context(), repo, s.env); err == nil || copied == "" {
			t.Fatalf("%s: failed listing passed or was not reached: %v", mode, err)
		}
		if _, err := os.Lstat(filepath.Dir(copied)); !os.IsNotExist(err) {
			t.Fatalf("%s: failed listing retained its copy: %v", mode, err)
		}
		if !maps.Equal(before, m2InstalledIndexBytes(t, paths)) {
			t.Fatalf("%s: failed listing changed an original index", mode)
		}
	}
}

func TestM2InstalledIndexAcceptsEmptyListing(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	repo := m2SnapshotRepository(t, s)
	path := filepath.Join(repo, gitDirectory, "empty")
	env := m2IndexTestEnvironment(s.env, path)
	got := s.execute(t.Context(), command{program: s.git, args: []string{"read-tree", "--empty"}, dir: repo, env: env})
	if err := success(got); err != nil {
		t.Fatal(err)
	}
	before := m2InstalledIndexBytes(t, []string{path})
	entries, err := s.m2IndexEntries(t.Context(), repo, env, path)
	if err != nil || entries != "" || !maps.Equal(before, m2InstalledIndexBytes(t, []string{path})) {
		t.Fatalf("empty index listing = %q, error = %v", entries, err)
	}
}

func TestM2InstalledObservationPreparationFailures(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	repo := m2SnapshotRepository(t, s)
	fixture := s.execute
	s.execute = func(context.Context, command) result { return result{err: ErrAcceptance} }
	if _, err := s.captureM2Installed(t.Context(), repo, s.env); err == nil {
		t.Fatal("failed initial snapshot passed")
	}
	s.execute = fixture
	if _, err := s.m2IndexEntries(t.Context(), repo, s.env, filepath.Join(repo, "missing")); err == nil {
		t.Fatal("missing original index passed")
	}
	s.base = filepath.Join(repo, pathNote)
	if _, err := s.m2IndexEntries(t.Context(), repo, s.env, filepath.Join(repo, gitDirectory, "index")); err == nil {
		t.Fatal("invalid copied-index directory passed")
	}
	if _, err := s.m2IndexEntriesAt(t.Context(), repo, s.env, nil, s.base); err == nil {
		t.Fatal("failed copied-index write passed")
	}
}

func TestM2InstalledIndexCleanupFailureIsReported(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == goosWindows {
		t.Skip("Unix permission faults require mode enforcement")
	}
	s := m2TestSuite(t)
	repo := m2SnapshotRepository(t, s)
	fixture := s.execute
	s.execute = func(ctx context.Context, c command) result {
		got := fixture(ctx, c)
		dir := filepath.Dir(environmentValue(c.env, "GIT_INDEX_FILE"))
		t.Cleanup(func() {
			if err := os.Chmod(dir, dirMode); err != nil {
				t.Error(err)
			}
		})
		if err := os.Chmod(dir, 0); err != nil {
			t.Fatal(err)
		}
		return got
	}
	if _, err := s.m2IndexEntries(t.Context(), repo, s.env, filepath.Join(repo, gitDirectory, "index")); err == nil {
		t.Fatal("failed copied-index cleanup passed")
	}
}
