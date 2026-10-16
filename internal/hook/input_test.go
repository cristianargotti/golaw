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
		strings.Repeat(" ", (1<<20)+1) + "{}",
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
}

func TestRelativeAndHash(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	got, err := hook.Relative(root, root, filepath.Join(root, "dir", "a.go"))
	if err != nil || got != "dir/a.go" {
		t.Fatalf("got %q, %v", got, err)
	}
	for _, name := range []string{"", ".", "../outside.go"} {
		if _, err := hook.Relative(root, root, name); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("symbolic links are not available: %v", err)
	}
	if got, err := hook.Relative(root, link, "a.go"); err != nil || got != "a.go" {
		t.Fatalf("linked cwd: %q, %v", got, err)
	}
	if got, err := hook.Relative(root, root, filepath.Join(link, "dir", "b.go")); err != nil || got != "dir/b.go" {
		t.Fatalf("linked name: %q, %v", got, err)
	}
	if got, err := hook.Relative(root, link, filepath.Join("absent", "deep", "c.go")); err != nil || got != "absent/deep/c.go" {
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
		if got, err := hook.Relative(repo, outside, name); err != nil || got != want {
			t.Errorf("%s: %q, %v, want %q", name, got, err, want)
		}
	}
	for _, name := range []string{filepath.Join(repo, "out.md"), filepath.Join(repo, "gone.md")} {
		if got, err := hook.Relative(repo, repo, name); !errors.Is(err, model.ErrCannotJudge) {
			t.Errorf("%s: %q, %v, want outside the repository", name, got, err)
		}
	}
}
