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
