package model_test

import (
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestCommitValidateAcceptsWhatGitWrites(t *testing.T) {
	t.Parallel()
	valid := []model.Commit{
		{Message: "feat: add the rung\n"},
		{Message: "", Cleanup: model.CleanupVerbatim},
		{Message: "fix: y", Cleanup: model.CleanupWhitespace, CommentChar: ";"},
		{Message: "x", Cleanup: model.CleanupScissors, CommentChar: model.DefaultCommentChar},
		{Message: "x", Cleanup: model.CleanupStrip, CommentChar: "//", Author: "tester <tester@example.com>"},
		{Message: "x", CommentChar: " "},
		{Message: "x", CommentChar: "\t#"},
		{Message: "x", CommentChar: "\r"},
	}
	for _, c := range valid {
		if err := c.Validate(); err != nil {
			t.Errorf("%+v: %v", c, err)
		}
	}
}

func TestCommitValidateRefusesWhatGitNeverWrites(t *testing.T) {
	t.Parallel()
	invalid := map[string]model.Commit{
		"unresolved default": {Message: "x", Cleanup: "default"},
		"unknown cleanup":    {Message: "x", Cleanup: "trim"},
		"line break":         {Message: "x", CommentChar: "#\n"},
	}
	for name, c := range invalid {
		if err := c.Validate(); !errors.Is(err, model.ErrUsage) {
			t.Errorf("%s: %v, want ErrUsage", name, err)
		}
	}
}

func TestInputCarriesTheCommitOfCommitMode(t *testing.T) {
	t.Parallel()
	commit := &model.Commit{Message: "docs: say why\n"}
	in := model.Input{Mode: model.ModeCommit, Commit: commit}
	if in.Commit != commit || in.Event != nil {
		t.Errorf("input = %+v", in)
	}
}
