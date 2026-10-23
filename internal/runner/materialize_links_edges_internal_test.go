package runner

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestTreeLinksResolveTheCompletedTree(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		links map[string]string
		bad   bool
	}{
		{"red-later-link-escape", map[string]string{"link": "later/../outside", "later": "."}, true},
		{"red-repeated-link-escape", map[string]string{"link": "later/../later/../../outside", "later": "dir"}, true},
		{"near-repeated-link", map[string]string{"link": "later/../later/file", "later": "dir"}, false},
		{"near-later-link", map[string]string{"link": "later/file", "later": "dir"}, false},
		{"near-cycle", map[string]string{"link": "later", "later": "link"}, false},
		{"near-nested-cycle", map[string]string{"link": "dir/nested", "dir/nested": "../link"}, false},
		{"near-dangling", map[string]string{"link": "missing/child"}, false},
		{"near-not-directory", map[string]string{"link": "dir/file/child"}, false},
		{"near-file-before-parent", map[string]string{"link": "dir/file/../../../outside"}, false},
		{"near-link-file-before-parent", map[string]string{"link": "later/../../../outside", "later": "dir/file"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			requireTreeLinks(t, tc.links, tc.bad)
		})
	}
}

func requireTreeLinks(t *testing.T, links map[string]string, bad bool) {
	t.Helper()
	root := linkPayload(t, links)
	err := createTreeLinks(root, linkEntries(links))
	if bad || treeLinkHostLimit(t, links) {
		requireTreeLinkLimit(t, err, links)
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range links {
		if got, readErr := root.Readlink(name); readErr != nil || got != filepath.FromSlash(want) {
			t.Fatalf("link %q = %q, %v, want %q", name, got, readErr, want)
		}
	}
}

func treeLinkHostLimit(t *testing.T, links map[string]string) bool {
	t.Helper()
	for _, target := range links {
		name := filepath.Join(t.TempDir(), "probe")
		if err := os.Symlink(target, name); err != nil {
			return true
		}
		got, err := os.Readlink(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != filepath.FromSlash(target) { // Windows stores a target with its own separator
			return true
		}
	}
	return false
}

func requireTreeLinkLimit(t *testing.T, err error, links map[string]string) {
	t.Helper()
	if !errors.Is(err, model.ErrCannotJudge) {
		t.Fatalf("limitation = %v", err)
	}
	for name := range links {
		if strings.Contains(err.Error(), strconv.Quote(name)) {
			return
		}
	}
	t.Fatalf("limitation does not name the link: %v", err)
}

func linkPayload(t *testing.T, links map[string]string) *os.Root {
	t.Helper()
	dir := t.TempDir()
	if err := os.Symlink("target", filepath.Join(dir, "probe")); err != nil {
		t.Skipf("host cannot create symbolic links: %v", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := root.Mkdir("dir", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFile("dir/file", []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, target := range links {
		if err := root.WriteFile(name, []byte(target), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func linkEntries(links map[string]string) []treeEntry {
	entries := make([]treeEntry, 0, len(links))
	for name, target := range links {
		entries = append(entries, treeEntry{path: name, mode: "120000", size: int64(len(target))})
	}
	return entries
}

func TestTreeLinksNameTargetLimits(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, target string }{
		{"red-absolute", filepath.Join(t.TempDir(), "outside")},
		{"red-host-capacity", strings.Repeat("x", (64<<10)+1)},
		{"red-nul", "missing\x00suffix"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			links := map[string]string{"link": tc.target}
			err := createTreeLinks(linkPayload(t, links), linkEntries(links))
			if !errors.Is(err, model.ErrCannotJudge) || !strings.Contains(err.Error(), "link") {
				t.Fatalf("limitation = %v", err)
			}
		})
	}
}

func TestTreeLinksPreservePayloadFilesystemFailure(t *testing.T) {
	t.Parallel()
	root := linkPayload(t, nil)
	err := createTreeLinks(root, []treeEntry{{path: "missing", mode: "120000"}})
	if !errors.Is(err, os.ErrNotExist) || errors.Is(err, model.ErrCannotJudge) || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("payload failure = %v", err)
	}
}

func TestTreeLinksHandleTheHostEmptyTarget(t *testing.T) {
	t.Parallel()
	probeErr := os.Symlink("", filepath.Join(t.TempDir(), "probe"))
	links := map[string]string{"link": "later/../outside", "later": ""}
	root := linkPayload(t, links)
	err := createTreeLinks(root, linkEntries(links))
	if probeErr != nil {
		if !errors.Is(err, model.ErrCannotJudge) || !strings.Contains(err.Error(), "later") {
			t.Fatalf("empty target limitation = %v", err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if got, readErr := root.Readlink("later"); readErr != nil || got != "" {
		t.Fatalf("empty link = %q, %v", got, readErr)
	}
}
