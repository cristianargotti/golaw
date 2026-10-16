package bootstrap_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
)

// repo creates a git checkout root under a temporary directory and returns
// its real path, so the symbolic link of the temporary directory on macOS
// never enters the comparison.
func repo(t *testing.T, entries ...string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o750); err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		path := filepath.Join(root, filepath.FromSlash(entry))
		if filepath.Ext(entry) == "" {
			if err := os.MkdirAll(path, 0o750); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestRootResolvesTheCheckoutAndRefusesTheRest(t *testing.T) {
	t.Parallel()
	root := repo(t)
	got, err := bootstrap.Root(root)
	if err != nil || got != root {
		t.Fatalf("root = %q, %v", got, err)
	}
	for name, path := range map[string]string{"missing": filepath.Join(root, "nope"), "no git": t.TempDir()} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := bootstrap.Root(path); !errors.Is(err, bootstrap.ErrNotRepository) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestDetectReadsTheLayout(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		entries []string
		want    bootstrap.Layout
	}{
		"empty":        {nil, bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/adr"}},
		"go at root":   {[]string{"go.mod", "main.go", "cmd"}, bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/adr", Go: true}},
		"go under cmd": {[]string{"go.mod", "cmd"}, bootstrap.Layout{Main: bootstrap.MainCmd, Decisions: "docs/adr", Go: true}},
		"adr":          {[]string{"docs/adr"}, bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/adr"}},
		"decisions":    {[]string{"docs/decisions"}, bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/decisions"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := bootstrap.Detect(repo(t, tc.entries...))
			if err != nil || got != tc.want {
				t.Fatalf("layout = %+v, %v, want %+v", got, err, tc.want)
			}
		})
	}
}

func TestDetectRefusesTwoDecisionDirectories(t *testing.T) {
	t.Parallel()
	if _, err := bootstrap.Detect(repo(t, "docs/adr", "docs/decisions")); !errors.Is(err, bootstrap.ErrDecisions) {
		t.Fatalf("err = %v", err)
	}
}

func TestCurrentAndWriteRoundTrip(t *testing.T) {
	t.Parallel()
	root := repo(t)
	if got, err := bootstrap.Current(root); got != nil || err != nil {
		t.Fatalf("absent = %q, %v", got, err)
	}
	if err := bootstrap.Write(root, []byte("golaw: 1\n")); err != nil {
		t.Fatal(err)
	}
	if err := bootstrap.Write(root, []byte("golaw: 1\nlanguage: pt-BR\n")); err != nil {
		t.Fatal(err)
	}
	got, err := bootstrap.Current(root)
	if err != nil || string(got) != "golaw: 1\nlanguage: pt-BR\n" {
		t.Fatalf("current = %q, %v", got, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("a temporary file survived: %v", entries)
	}
}

func TestCurrentAndWriteRefuseASymlink(t *testing.T) {
	t.Parallel()
	root := repo(t, "elsewhere.yml")
	if err := os.Symlink(filepath.Join(root, "elsewhere.yml"), filepath.Join(root, bootstrap.PolicyFile)); err != nil {
		t.Skip("symbolic links are not available here:", err)
	}
	if _, err := bootstrap.Current(root); !errors.Is(err, bootstrap.ErrSymlink) {
		t.Fatalf("current err = %v", err)
	}
	if err := bootstrap.Write(root, []byte("x")); !errors.Is(err, bootstrap.ErrSymlink) {
		t.Fatalf("write err = %v", err)
	}
}

func TestCurrentAndWriteReportWhatTheyCannotDo(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "absent")
	if err := bootstrap.Write(missing, []byte("x")); err == nil || errors.Is(err, bootstrap.ErrSymlink) {
		t.Fatalf("write into a missing root = %v", err)
	}
	asDirectory := repo(t, "golaw.yml/keep.txt")
	if err := bootstrap.Write(asDirectory, []byte("x")); err == nil || !strings.Contains(err.Error(), "publish golaw.yml") {
		t.Fatalf("write over a directory = %v", err)
	}
	if _, err := bootstrap.Current(asDirectory); err == nil || !strings.Contains(err.Error(), "read golaw.yml") {
		t.Fatalf("current of a directory = %v", err)
	}
	entries, err := os.ReadDir(asDirectory)
	if err != nil || len(entries) != 2 {
		t.Fatalf("a temporary file survived the failed publish: %v, %v", entries, err)
	}
	fileAsRoot := filepath.Join(repo(t), ".git", "config")
	if err := os.WriteFile(fileAsRoot, []byte("[core]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := bootstrap.Current(fileAsRoot); err == nil || !strings.Contains(err.Error(), "inspect golaw.yml") {
		t.Fatalf("current under a file = %v", err)
	}
}
