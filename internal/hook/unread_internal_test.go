package hook

import (
	"errors"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func paths(files []model.File) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

// TestUnreadViewHoldsAWriteOverTheBound reads the paths of a write over
// the bound as over the bound, in place of what the base held there or
// beside it, and answers every other path from the base.
func TestUnreadViewHoldsAWriteOverTheBound(t *testing.T) {
	t.Parallel()
	base := proof.NewTree(map[string][]byte{"a.go": []byte("package a\n"), "old.md": []byte("old\n")})
	view := unreadView{base: base, files: map[string]model.File{
		"old.md": overBound(base, "old.md"), "new.md": overBound(base, "new.md"),
	}}
	if got := paths(view.Files()); !slices.Equal(got, []string{"a.go", "new.md", "old.md"}) {
		t.Fatalf("Files = %v", got)
	}
	if got := paths(view.Select(func(p string) bool { return p != "a.go" })); !slices.Equal(got, []string{"new.md", "old.md"}) {
		t.Fatalf("Select = %v", got)
	}
	if !view.Any(func(p string) bool { return p == "new.md" }) || !view.Any(func(p string) bool { return p == "a.go" }) || view.Any(func(p string) bool { return p == "none" }) {
		t.Fatal("Any does not answer the view")
	}
	if _, err := view.Read("old.md"); !errors.Is(err, model.ErrTooLarge) {
		t.Fatalf("a path of the write reads %v", err)
	}
	if data, err := view.Read("a.go"); err != nil || string(data) != "package a\n" {
		t.Fatalf("a path of the base reads %q, %v", data, err)
	}
	if view.Class("new.md") != model.ClassUntracked || view.Class("a.go") != base.Class("a.go") || view.files["old.md"].Size <= model.MaxReadBytes {
		t.Fatal("the classes and sizes of the view")
	}
	if got := withUnread([]Gap{OverReadBound("old.md")}, []string{"old.md", "a.go"}); len(got) != 2 || got[1].Path != "a.go" {
		t.Fatalf("withUnread = %+v", got)
	}
}
