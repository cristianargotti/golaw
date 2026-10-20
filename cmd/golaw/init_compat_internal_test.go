package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestInitUnloadableReviewedPolicyKeepsM1Reason(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	const reviewed = "golaw: 1\nkit: { version: 9.9.9 }\nunknown: true\n"
	repo.Write("go.mod", "module example.test/service\n\ngo 1.27\n")
	repo.Write(bootstrap.PolicyFile, reviewed)
	ta := initBindApp(t)
	if code := ta.run(t.Context(), []string{"init", "--apply", repo.Root}); code != exitFailure {
		t.Fatalf("unloadable policy exit = %d", code)
	}
	want := "golaw.yml of " + repo.Root + " cannot be loaded, so init neither replaces nor enrolls it"
	if !strings.Contains(ta.stderr.String(), want) {
		t.Errorf("reviewed-file reason missing: %q", ta.stderr.String())
	}
	after, err := bootstrap.Current(repo.Root)
	if err != nil || string(after) != reviewed || ta.stdout.Len() != 0 {
		t.Fatalf("unloadable policy changed: %q, %v, stdout %q", after, err, ta.stdout.String())
	}
	entries, err := os.ReadDir(repo.Root)
	if err != nil || len(entries) != 3 {
		t.Fatalf("initialization wrote files: %+v, %v", entries, err)
	}
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	if _, found, readErr := policy.ReadRecord(dirs.State, repo.Root); readErr != nil || found {
		t.Fatalf("unloadable policy enrollment = %v, %v", found, readErr)
	}
}

func TestInitUnchangedPolicyKeepsM1Status(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, language, status string
		tightened              bool
	}{
		{"defaults", "pt-BR", "golaw.yml: no changes\n", false},
		{"same flags", "en-US", "golaw.yml: no changes\n", false},
		{"reviewed floor", "pt-BR", "golaw.yml: kept as reviewed, init enrolls it and renders nothing over it\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			requireUnchangedPolicyStatus(t, tc.language, tc.status, tc.tightened)
		})
	}
}

func requireUnchangedPolicyStatus(t *testing.T, language, status string, tightened bool) {
	t.Helper()
	repo := gittest.New(t)
	repo.Write("go.mod", "module example.test/service\n\ngo 1.27\n")
	ta := initBindApp(t)
	flags := []string{"--language", language, "--capabilities", "go,api"}
	first := append([]string{"init", "--apply", repo.Root}, flags...)
	if code := ta.run(t.Context(), first); code != exitClean {
		t.Fatalf("first apply = %d: %s", code, ta.stderr.String())
	}
	current, err := bootstrap.Current(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	if tightened {
		current = bytes.Replace(current, []byte("file_lines: 300"), []byte("file_lines: 200"), 1)
		if writeErr := os.WriteFile(filepath.Join(repo.Root, bootstrap.PolicyFile), current, 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	times := managedTimes(t, repo.Root)
	for _, apply := range []bool{false, true} {
		args := append([]string{"init", repo.Root}, flags...)
		if apply {
			args = append(args, "--apply")
		}
		ta.stdout.Reset()
		if code := ta.run(t.Context(), args); code != exitClean {
			t.Fatalf("repeat init = %d: %s", code, ta.stderr.String())
		}
		if !strings.HasPrefix(ta.stdout.String(), status) {
			t.Errorf("policy status missing or not first: %q", ta.stdout.String())
		}
		requireSilentApply(t, ta.stdout.String(), repo.Root, times)
		after, readErr := bootstrap.Current(repo.Root)
		if readErr != nil || !bytes.Equal(after, current) {
			t.Fatalf("unchanged policy rewritten: %q, %v", after, readErr)
		}
	}
}
