package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
)

func initBindApp(t *testing.T) testApp {
	t.Helper()
	ta := bindApp(t)
	b := ta.bind()
	ta.compile = b.compile
	ta.enroll = b.enroll
	ta.committed = b.committed
	return ta
}

func TestInitTemplatesRequireApplyAndRemainUnarmed(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	module := "module example.test/service\n\ngo 1.27\n\nrequire (\n example.test/library v1.2.3\n github.com/quasilyte/go-ruleguard/dsl v0.3.23\n)\n"
	makefile := "all:\n\tgo build ./...\n\ninclude golaw.mk\n"
	repo.Commit("chore: establish application", map[string]string{
		"go.mod": module, "Makefile": makefile,
	})
	ta := initBindApp(t)
	requireDryRun(t, ta, repo.Root)
	ta.stdout.Reset()
	if code := ta.run(t.Context(), []string{"init", "--apply", repo.Root}); code != exitClean {
		t.Fatalf("apply exit = %d: %s", code, ta.stderr.String())
	}
	times := managedTimes(t, repo.Root)
	requireUnarmed(t, repo.GitDir())
	ta.stdout.Reset()
	if code := ta.run(t.Context(), []string{"init", "--apply", repo.Root}); code != exitClean {
		t.Fatalf("second apply exit = %d: %s", code, ta.stderr.String())
	}
	requireSilentApply(t, ta.stdout.String(), repo.Root, times)
	for name, want := range map[string]string{"go.mod": module, "Makefile": makefile} {
		got, err := os.ReadFile(filepath.Join(repo.Root, name))
		if err != nil || string(got) != want {
			t.Fatalf("person-owned file changed: %s: %q, %v", name, got, err)
		}
	}
}

// requireDryRun requires init without --apply to print the template diff
// and to write nothing.
func requireDryRun(t *testing.T, ta testApp, root string) {
	t.Helper()
	if code := ta.run(t.Context(), []string{"init", root}); code != exitClean {
		t.Fatalf("dry run exit = %d: %s", code, ta.stderr.String())
	}
	if !strings.Contains(ta.stdout.String(), "+++ lefthook.yml\n") {
		t.Fatalf("template diff missing: %q", ta.stdout.String())
	}
	for _, name := range []string{"golaw.yml", "lefthook.yml", "golaw.mk"} {
		if _, err := os.Stat(filepath.Join(root, name)); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("dry run wrote %s: %v", name, err)
		}
	}
}

// managedTimes requires every file the managed list of golaw.yml records
// to hold its recorded hash and to be one init owns, and answers the file
// information of each.
func managedTimes(t *testing.T, root string) map[string]fs.FileInfo {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "golaw.yml"))
	if err != nil {
		t.Fatal(err)
	}
	config, err := policy.Decode(data)
	if err != nil || len(config.Managed) == 0 {
		t.Fatalf("managed policy = %+v, %v", config, err)
	}
	times := make(map[string]fs.FileInfo)
	for _, managed := range config.Managed {
		path := filepath.Join(root, filepath.FromSlash(managed.Path))
		content, readErr := os.ReadFile(path)
		if readErr != nil || install.Digest(content) != managed.SHA256 {
			t.Fatalf("managed hash: %s: %v", managed.Path, readErr)
		}
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatal(statErr)
		}
		times[managed.Path] = info
		if managed.Path == "Makefile" || strings.HasPrefix(managed.Path, "docs/") {
			t.Fatalf("person-owned file recorded as managed: %s", managed.Path)
		}
	}
	return times
}

// requireUnarmed requires init to leave every rung hook unwritten.
func requireUnarmed(t *testing.T, gitDir string) {
	t.Helper()
	for _, hook := range []string{"pre-commit", "commit-msg", "pre-push"} {
		if _, err := os.Stat(filepath.Join(gitDir, "hooks", hook)); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("init armed %s: %v", hook, err)
		}
	}
}

// requireSilentApply requires a second apply to print no proposal or
// write and to leave every managed file untouched.
func requireSilentApply(t *testing.T, out, root string, times map[string]fs.FileInfo) {
	t.Helper()
	for _, marker := range []string{"+++ ", "--- ", "Gravações:", "Propostas sem gravação:"} {
		if strings.Contains(out, marker) {
			t.Fatalf("second apply has a proposal or write: %q", out)
		}
	}
	for name, before := range times {
		after, err := os.Stat(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || !after.ModTime().Equal(before.ModTime()) {
			t.Fatalf("second apply rewrote %s: %v", name, err)
		}
	}
}
