package main

import (
	"context"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestM2InstalledAllowsIndexStatRefresh(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"installed-pre-commit-red", "installed-pre-commit-near",
		"spawn-git-red", "spawn-git-near",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := m2TestSuite(t)
			script := m2NativeScript(t, s, name, "answer")
			s.execute = func(ctx context.Context, c command) result {
				got := script.Execute(ctx, c)
				if m2InstalledHook(s, c) {
					m2RefreshInstalledIndexes(ctx, t, s, c)
				}
				return got
			}
			if got := s.nativeM2(t.Context(), name); got.status != statusPass || script.Targets != 1 {
				t.Fatalf("%s: target=%d result=%+v", name, script.Targets, got)
			}
		})
	}
}

func m2RefreshInstalledIndexes(ctx context.Context, t *testing.T, s *suite, c command) {
	t.Helper()
	gitDir, err := s.m2ObserveGit(ctx, c.dir, c.env, "rev-parse", "--absolute-git-dir")
	if err != nil {
		t.Fatal(err)
	}
	before, _, err := m2ReadIndexes(c.dir, gitDir, c.env)
	if err != nil {
		t.Fatal(err)
	}
	for path := range before {
		env := m2IndexTestEnvironment(c.env, path)
		listing := s.execute(ctx, command{
			program: s.git, args: []string{"ls-files", "--stage", "-z"}, dir: c.dir, env: env,
		})
		if listErr := success(listing); listErr != nil {
			t.Fatal(listErr)
		}
		got := s.execute(ctx, command{
			program: s.git, args: []string{"update-index", "-z", "--index-info"},
			dir: c.dir, env: env, input: listing.stdout,
		})
		if updateErr := success(got); updateErr != nil {
			t.Fatal(updateErr)
		}
	}
	after, _, err := m2ReadIndexes(c.dir, gitDir, c.env)
	if err != nil || maps.Equal(before, after) {
		t.Fatalf("index stat refresh did not change raw bytes: %v", err)
	}
}

func m2IndexTestEnvironment(env []string, path string) []string {
	env = slices.DeleteFunc(slices.Clone(env), func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		return strings.EqualFold(key, "GIT_INDEX_FILE")
	})
	return append(env, "GIT_INDEX_FILE="+path)
}

func TestM2InstalledRejectsDefaultIndexEntryChanges(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"mode", "object", "path", "stage"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			s := m2TestSuite(t)
			script := m2NativeScript(t, s, "spawn-git-near", "answer")
			s.execute = func(ctx context.Context, c command) result {
				got := script.Execute(ctx, c)
				if m2InstalledHook(s, c) {
					m2ChangeInstalledEntry(ctx, t, s, c, mode)
				}
				return got
			}
			if got := s.nativeM2(t.Context(), "spawn-git-near"); got.status != statusFail || script.Targets != 1 {
				t.Fatalf("%s: target=%d result=%+v", mode, script.Targets, got)
			}
		})
	}
}

func m2ChangeInstalledEntry(ctx context.Context, t *testing.T, s *suite, c command, mode string) {
	t.Helper()
	gitDir, err := s.m2ObserveGit(ctx, c.dir, c.env, "rev-parse", "--absolute-git-dir")
	if err != nil {
		t.Fatal(err)
	}
	env := m2IndexTestEnvironment(c.env, filepath.Join(gitDir, "index"))
	entry, err := s.m2ObserveGit(ctx, c.dir, env, "ls-files", "--stage", "--", pathNote)
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(entry)
	switch mode {
	case "mode":
		fields[0] = "100755"
	case "object":
		fields[1], err = s.m2ObserveGit(ctx, c.dir, env, "rev-parse", "HEAD:go.mod")
	case "path":
		fields[3] = "renamed.txt"
	case "stage":
		fields[2] = "1"
	}
	if err != nil {
		t.Fatal(err)
	}
	got := s.execute(ctx, command{
		program: s.git, args: []string{"update-index", "--index-info"}, dir: c.dir, env: env,
		input: strings.Join(fields[:3], " ") + "\t" + fields[3] + "\n",
	})
	if err := success(got); err != nil {
		t.Fatal(err)
	}
}
