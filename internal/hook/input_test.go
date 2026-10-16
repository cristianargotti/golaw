package hook_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestReadObject(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		"", "{", "null", "[]", "{}{}", `{"x":1} false`,
		strings.Repeat(" ", hook.MaxInputBytes+1) + "{}",
	} {
		if _, err := hook.ReadObject(strings.NewReader(input)); !errors.Is(err, model.ErrCannotJudge) {
			t.Fatalf("%q: %v", input[:min(len(input), 20)], err)
		}
	}
	input := " \n{\"value\":\"safe\"}\n"
	got, err := hook.ReadObject(strings.NewReader(input))
	if err != nil || string(got) != input {
		t.Fatalf("got %q, %v", got, err)
	}
	large := strings.Repeat(" ", (1<<20)+1) + "{}"
	if got, err := hook.ReadObject(strings.NewReader(large)); err != nil || len(got) != len(large) {
		t.Fatalf("an input past the earlier bound of 1 MiB: %d bytes, %v", len(got), err)
	}
	over := strings.Repeat(" ", hook.MaxInputBytes) + "{}"
	if prefix, err := hook.ReadObject(strings.NewReader(over)); !errors.Is(err, model.ErrTooLarge) || len(prefix) != hook.MaxInputBytes+1 {
		t.Fatalf("an input over the bound answers its prefix: %d bytes, %v", len(prefix), err)
	}
}

func TestRelativeAndHash(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	got, err := (hook.Base{Root: root}).Relative(root, filepath.Join(root, "dir", "a.go"))
	if err != nil || got != "dir/a.go" {
		t.Fatalf("got %q, %v", got, err)
	}
	for _, name := range []string{"", ".", "../outside.go"} {
		if _, err := (hook.Base{Root: root}).Relative(root, name); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("symbolic links are not available: %v", err)
	}
	if got, err := (hook.Base{Root: root}).Relative(link, "a.go"); err != nil || got != "a.go" {
		t.Fatalf("linked cwd: %q, %v", got, err)
	}
	if got, err := (hook.Base{Root: root}).Relative(root, filepath.Join(link, "dir", "b.go")); err != nil || got != "dir/b.go" {
		t.Fatalf("linked name: %q, %v", got, err)
	}
	if got, err := (hook.Base{Root: root}).Relative(link, filepath.Join("absent", "deep", "c.go")); err != nil || got != "absent/deep/c.go" {
		t.Fatalf("absent directories: %q, %v", got, err)
	}
	if hook.Hash(nil) !=
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatal("unexpected SHA-256")
	}
	if hook.Hash([]byte("a")) == hook.Hash([]byte("b")) {
		t.Fatal("different content has the same identity")
	}
}

// TestRelativeFollowsEveryLink names a candidate by the file its name
// leads to through every link, the last one included and dangling or not,
// with the parent element of a link text taken from where the link lands;
// a link that leads out of the repository is outside it. The name is read
// the way the host expands it, trimmed of the white space its JavaScript
// trim removes, which keeps U+0085.
func TestRelativeFollowsEveryLink(t *testing.T) {
	t.Parallel()
	repo, outside := realDir(t), realDir(t)
	touch(t, filepath.Join(repo, "golaw.yml"))
	touch(t, filepath.Join(repo, "sub", "keep.md"))
	touch(t, filepath.Join(outside, "plain.md"))
	for _, pair := range [][2]string{
		{filepath.Join(repo, "golaw.yml"), filepath.Join(outside, "alias")},
		{filepath.Join(repo, "fresh.md"), filepath.Join(outside, "dangling.md")},
		{filepath.Join(repo, "sub"), filepath.Join(outside, "hop")},
		{strings.Join([]string{"hop", "..", "golaw.yml"}, string(filepath.Separator)), filepath.Join(outside, "twist")},
		{filepath.Join(outside, "plain.md"), filepath.Join(repo, "out.md")},
		{filepath.Join(outside, "absent.md"), filepath.Join(repo, "gone.md")},
	} {
		link(t, pair[0], pair[1])
	}
	for name, want := range map[string]string{
		filepath.Join(outside, "alias"):                      "golaw.yml",
		filepath.Join(outside, "dangling.md"):                "fresh.md",
		filepath.Join(outside, "twist"):                      "golaw.yml",
		filepath.Join(outside, "hop", "x.md"):                "sub/x.md",
		" \uFEFF" + filepath.Join(outside, "alias") + "\n\t": "golaw.yml",
		filepath.Join(repo, "golaw.yml") + "\u0085":          "golaw.yml\u0085",
	} {
		if got, err := (hook.Base{Root: repo}).Relative(outside, name); err != nil || got != want {
			t.Errorf("%s: %q, %v, want %q", name, got, err, want)
		}
	}
	for _, name := range []string{filepath.Join(repo, "out.md"), filepath.Join(repo, "gone.md")} {
		if got, err := (hook.Base{Root: repo}).Relative(repo, name); !errors.Is(err, model.ErrCannotJudge) {
			t.Errorf("%s: %q, %v, want outside the repository", name, got, err)
		}
	}
}

// TestBaseNamesAToolPathFromTheHome names a candidate the host opens from
// the home from the root, wherever the session runs, refuses one whose
// home the process does not know, and resolves such a name under the
// working directory when it can only be judged by its spelling.
func TestBaseNamesAToolPathFromTheHome(t *testing.T) {
	t.Parallel()
	base := realDir(t)
	home, repo, outside := filepath.Join(base, "home"), filepath.Join(base, "repo"), filepath.Join(base, "outside")
	for _, dir := range []string{home, repo, outside} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	at := hook.Base{Root: repo, Home: home}
	if got, err := at.Relative(outside, "~/../repo/a.go"); err != nil || got != "a.go" {
		t.Fatalf("Relative: %q, %v", got, err)
	}
	if got, err := at.Requested(outside, "~/../repo/docs/a.md"); err != nil || got != "docs/a.md" {
		t.Fatalf("Requested: %q, %v", got, err)
	}
	if got := at.Resolve(outside, "~/../outside/x.md"); got != filepath.Join(outside, "x.md") {
		t.Fatalf("Resolve: %q", got)
	}
	unknown := hook.Base{Root: repo}
	if got, err := unknown.Relative(outside, "~/../repo/a.go"); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatalf("Relative from an unknown home: %q, %v", got, err)
	}
	if got := unknown.Resolve(outside, "~/x.md"); got != filepath.Join(outside, "~", "x.md") {
		t.Fatalf("Resolve from an unknown home: %q", got)
	}
}
