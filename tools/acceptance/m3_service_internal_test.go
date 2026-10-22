package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// m3ServiceAnswer answers init with the M2 simulation, which renders the
// core templates and enrolls, and go get and go mod tidy with success,
// recording each.
func m3ServiceAnswer(t *testing.T, s *suite, calls *[][]string) m2Answer {
	t.Helper()
	return func(_ context.Context, c command) (m2Reply, bool) {
		switch {
		case c.program == s.binary && len(c.args) > 0 && c.args[0] == commandInit:
			*calls = append(*calls, c.args)
			return m2Reply{got: m2Init(t, s, c), target: true}, true
		case c.program == s.goTool && len(c.args) > 0 && (c.args[0] == "get" || c.args[0] == "mod"):
			*calls = append(*calls, c.args)
			return m2Reply{}, true
		}
		return m2Reply{}, false
	}
}

func TestM3ServiceRepositoryAdoptsTheFixture(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	var calls [][]string
	s.execute = newM2Script(t, s, m3ServiceAnswer(t, s, &calls)).Execute
	repo, err := s.m3ServiceRepository(t.Context(), m3ServiceCase, []string{"api", "go"})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) == 0 || !slices.Equal(calls[0], []string{"init", "--apply", "--capabilities", "api,go"}) {
		t.Fatalf("calls = %q", calls)
	}
	for _, path := range []string{"internal/secret/secret.go", "golaw.yml", "golaw.mk", "api/openapi.yaml"} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	got := runCommand(t.Context(), command{program: s.git, args: []string{"status", "--porcelain"}, dir: repo, env: s.env})
	if got.err != nil || got.code != 0 || got.stdout != "" {
		t.Fatalf("the adoption is not committed: %+v", got)
	}
	copied := filepath.Join(t.TempDir(), "copy")
	if err := m3CopyTree(repo, copied); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"internal/server/server.go", ".git/HEAD"} {
		want, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(filepath.Join(copied, filepath.FromSlash(path))); err != nil || !bytes.Equal(got, want) {
			t.Fatalf("copy of %s = %q, %v", path, got, err)
		}
	}
}

func TestM3CopyTreeRefusesALink(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	checkedWrite(t, filepath.Join(source, "a.txt"), "a\n")
	if err := os.Symlink(filepath.Join(source, "a.txt"), filepath.Join(source, "link")); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
	if err := m3CopyTree(source, filepath.Join(t.TempDir(), "copy")); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("copy over a link = %v", err)
	}
}

func TestM3RequiredVersionReadsAGoMod(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "go.mod")
	checkedWrite(t, path, "module example.test/kit\n\ngo 1.27\n\nrequire (\n\tgithub.com/quasilyte/go-ruleguard/dsl v0.3.23\n)\n")
	if version, err := m3RequiredVersion(path, m3RuleguardModule); err != nil || version != "v0.3.23" {
		t.Fatalf("version = %q, %v", version, err)
	}
	if _, err := m3RequiredVersion(path, "example.test/absent"); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("an absent module = %v", err)
	}
	checkedWrite(t, path, "module\n")
	if _, err := m3RequiredVersion(path, m3RuleguardModule); err == nil {
		t.Fatal("a broken go.mod must fail")
	}
}
