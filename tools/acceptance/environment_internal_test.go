package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEnvironment(t *testing.T) {
	t.Parallel()
	home, bin := t.TempDir(), t.TempDir()
	env := isolatedEnvironment([]string{
		"PATH=/system", "GOCACHE=/cache", "SystemRoot=C:\\Windows",
		"OPENAI_API_KEY=private", "ANTHROPIC_API_KEY=private",
		"HOME=/private", "UNRELATED_SECRET=private",
	}, home, bin)
	if environmentValue(env, "HOME") != home ||
		environmentValue(env, "USERPROFILE") != home ||
		environmentValue(env, "GOCACHE") != "/cache" ||
		strings.Contains(strings.Join(env, "\n"), "private") {
		t.Fatal(env)
	}
	if !strings.HasPrefix(environmentValue(env, "PATH"), bin+string(os.PathListSeparator)) {
		t.Fatal(env)
	}
	if got := environmentValue(env, "GOFLAGS"); got != "-mod=readonly -modcacherw" {
		t.Fatalf("GOFLAGS = %q, want a module cache the home can drop", got)
	}
	if environmentValue(env, "GIT_CONFIG_COUNT") != "2" ||
		environmentValue(env, "GIT_CONFIG_KEY_0") != "gc.auto" || environmentValue(env, "GIT_CONFIG_VALUE_0") != "0" ||
		environmentValue(env, "GIT_CONFIG_KEY_1") != "maintenance.auto" || environmentValue(env, "GIT_CONFIG_VALUE_1") != "false" {
		t.Fatalf("git config = %q, want no automatic maintenance in the fixture repositories", env)
	}
	changed := prependPath(env, "first")
	if environmentValue(changed, "PATH") == environmentValue(env, "PATH") ||
		environmentValue(prependPath(nil, "only"), "PATH") != "only" {
		t.Fatal(changed)
	}
	want := "golaw"
	if runtime.GOOS == "windows" {
		want += ".exe"
	}
	if executable("golaw") != want || environmentValue(env, "missing") != "" {
		t.Fatal("unexpected environment lookup")
	}
}

func TestEnvironmentFilesAndRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "nested", "note")
	if err := writeFile(path, []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}
	body, err := readFile(path)
	if err != nil || string(body) != "safe" {
		t.Fatalf("%q %v", body, err)
	}
	if _, absentErr := readFile(filepath.Join(root, "absent")); absentErr == nil {
		t.Fatal("missing file was readable")
	}
	if childErr := writeFile(filepath.Join(path, "child"), nil, 0o600); childErr == nil {
		t.Fatal("file accepted as directory")
	}
	if dirErr := writeFile(root, nil, 0o600); dirErr == nil {
		t.Fatal("directory accepted as file")
	}
	if modErr := writeFile(filepath.Join(root, "go.mod"), []byte("module example.invalid/test\n"), 0o600); modErr != nil {
		t.Fatal(modErr)
	}
	got, err := moduleRoot(filepath.Dir(path))
	if err != nil || got != root {
		t.Fatalf("%s %v", got, err)
	}
	if _, err := moduleRoot(t.TempDir()); err == nil {
		t.Fatal("missing module accepted")
	}
}
