package model_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestFileClassNames(t *testing.T) {
	t.Parallel()
	names := map[model.FileClass]string{
		model.ClassUnknown: "unknown", model.ClassTracked: "tracked", model.ClassUntracked: "untracked",
		model.ClassTest: "test", model.ClassFixture: "fixture", model.ClassGenerated: "generated",
		model.ClassBinary: "binary", model.ClassSymlink: "symlink", model.ClassGitlink: "gitlink",
		model.ClassUnreadable: "unreadable", model.FileClass(-1): "unknown", model.FileClass(99): "unknown",
	}
	for class, want := range names {
		if class.String() != want {
			t.Errorf("FileClass(%d).String() = %q, want %q", int(class), class.String(), want)
		}
	}
	text := []model.FileClass{model.ClassTracked, model.ClassUntracked, model.ClassTest, model.ClassFixture, model.ClassGenerated}
	for _, class := range text {
		if !class.IsText() {
			t.Errorf("%s must be text", class)
		}
	}
	notText := []model.FileClass{model.ClassUnknown, model.ClassBinary, model.ClassSymlink, model.ClassGitlink, model.ClassUnreadable}
	for _, class := range notText {
		if class.IsText() {
			t.Errorf("%s must not be text", class)
		}
	}
}

func TestChangeOpNames(t *testing.T) {
	t.Parallel()
	names := map[model.ChangeOp]string{
		model.ChangeAdd: "add", model.ChangeModify: "modify", model.ChangeDelete: "delete",
		model.ChangeRename: "rename", model.ChangeOp(0): "unknown",
	}
	for op, want := range names {
		if op.String() != want {
			t.Errorf("ChangeOp(%d).String() = %q, want %q", int(op), op.String(), want)
		}
	}
	if model.MaxReadBytes != 8*1024*1024 {
		t.Errorf("MaxReadBytes = %d", model.MaxReadBytes)
	}
}

// plain is a snapshot that only lists, the shape every check could read
// before Selector existed.
type plain []model.File

func (p plain) Files() []model.File        { return p }
func (plain) Read(string) ([]byte, error)  { return nil, model.ErrNotInSnapshot }
func (plain) Class(string) model.FileClass { return model.ClassUnknown }

// selecting answers Select and Any itself and lists nothing.
type selecting struct{ files []model.File }

func (selecting) Files() []model.File                     { return nil }
func (selecting) Read(string) ([]byte, error)             { return nil, model.ErrNotInSnapshot }
func (selecting) Class(string) model.FileClass            { return model.ClassUnknown }
func (s selecting) Select(func(string) bool) []model.File { return s.files }
func (s selecting) Any(func(string) bool) bool            { return len(s.files) > 0 }

func TestSelectAndAnyAskTheSnapshotsOwnSelector(t *testing.T) {
	t.Parallel()
	listed := plain{{Path: "a/x.go"}, {Path: "b/y.go"}}
	under := func(p string) bool { return p == "b/y.go" }
	if got := model.Select(listed, under); len(got) != 1 || got[0].Path != "b/y.go" {
		t.Errorf("Select over a plain snapshot = %+v", got)
	}
	if !model.Any(listed, under) || model.Any(listed, func(string) bool { return false }) {
		t.Error("Any over a plain snapshot must filter its listing")
	}
	own := selecting{files: []model.File{{Path: "own"}}}
	if got := model.Select(own, under); len(got) != 1 || got[0].Path != "own" || !model.Any(own, under) {
		t.Errorf("Select through the snapshot's own selector = %+v", got)
	}
}
