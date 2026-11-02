package gen

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/model"
)

func overlayFixture(t *testing.T) *plan {
	t.Helper()
	p := newPlan(Input{})
	p.in.Snapshot = gentest.Snapshot(t, map[string][]byte{"a.go": []byte("package a\n"), "b.go": []byte("package a\n")})
	p.writes["a.go"] = Write{Path: "a.go", Content: []byte("package a\n\nvar A int\n"), Mode: WriteRegenerable}
	p.writes["b.go"] = Write{Path: "b.go", Mode: WriteDelete}
	p.writes["c.go"] = Write{Path: "c.go", Content: []byte("package a\n"), Mode: WriteOwned}
	return p
}

func TestOverlayWithoutWritesIsTheSnapshot(t *testing.T) {
	t.Parallel()
	p := newPlan(Input{})
	p.in.Snapshot = gentest.Snapshot(t, nil)
	if _, ok := p.overlay().(pending); ok {
		t.Error("a run with no pending write answered an overlay")
	}
}

func TestOverlayReadsThePendingWrites(t *testing.T) {
	t.Parallel()
	s := overlayFixture(t).overlay()
	if got, err := s.Read("a.go"); err != nil || !strings.Contains(string(got), "var A") {
		t.Errorf("a.go = %q, %v, want the pending bytes", got, err)
	}
	if _, err := s.Read("b.go"); !errors.Is(err, model.ErrNotInSnapshot) {
		t.Errorf("deleted b.go read = %v, want ErrNotInSnapshot", err)
	}
	if _, err := s.Read("d.go"); !errors.Is(err, model.ErrNotInSnapshot) {
		t.Errorf("absent d.go read = %v, want ErrNotInSnapshot", err)
	}
	classes := []model.FileClass{s.Class("a.go"), s.Class("b.go"), s.Class("c.go"), s.Class("d.go")}
	want := []model.FileClass{model.ClassTracked, model.ClassUnknown, model.ClassUntracked, model.ClassUnknown}
	if !slices.Equal(classes, want) {
		t.Errorf("classes = %v, want %v", classes, want)
	}
}

func TestOverlayListsTheCreationsAndDropsTheDeletions(t *testing.T) {
	t.Parallel()
	s := overlayFixture(t).overlay()
	listed := s.Files()
	files := make([]string, 0, len(listed))
	for _, f := range listed {
		files = append(files, f.Path)
	}
	if !slices.Equal(files, []string{"a.go", "c.go"}) {
		t.Errorf("files = %v, want a.go and c.go", files)
	}
	selected := model.Select(s, func(p string) bool { return p != "a.go" })
	if len(selected) != 1 || selected[0].Path != "c.go" || selected[0].Size != int64(len("package a\n")) {
		t.Errorf("select = %+v, want c.go alone with its size", selected)
	}
	if !model.Any(s, func(p string) bool { return p == "c.go" }) || !model.Any(s, func(p string) bool { return p == "a.go" }) {
		t.Error("any missed a pending creation or a base file")
	}
	if model.Any(s, func(p string) bool { return p == "d.go" }) {
		t.Error("any matched a path no one holds")
	}
}
