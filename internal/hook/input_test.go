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
