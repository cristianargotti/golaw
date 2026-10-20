package commits

import (
	"context"
	"fmt"
	"regexp"

	"github.com/cristianargotti/golaw/internal/model"
)

// The theme a commit that is not feat may carry, the limits shell.shape
// holds a command line commit to: at most this many staged files and this
// many new lines.
const (
	themeFiles = 40
	themeLines = 400
)

// subjectType reads the conventional type of a subject the way shell.shape
// reads it, so the two judge one commit alike.
var subjectType = regexp.MustCompile(`^\s*([a-z]+)(?:\([^)]*\))?!?:`)

// Shape is commit.shape: a commit whose type is not feat carries one theme,
// at most 40 new files and 400 new lines. It counts the changes the rung
// hands it, so what a merge in progress brought is left out before it, and
// leaves alone a message git writes itself.
func Shape() model.Check {
	return shapeRule{spec: spec("commit.shape")}
}

// shapeRule is no message rule, because it reads the staged changes and
// their blobs, which a message rule never sees.
type shapeRule struct {
	spec model.Spec
}

func (r shapeRule) Spec() model.Spec { return r.spec }

// Run counts the new files first and the new lines only when the files
// keep to their limit, so a commit of many new files reads no blob.
func (r shapeRule) Run(_ context.Context, in model.Input) (model.Result, error) {
	if in.Commit == nil {
		return model.Result{Status: model.NotApplicable(noCommit)}, nil
	}
	res := model.Result{Status: model.Measured(), Scope: fmt.Sprintf("%d changes", len(in.Changes))}
	kind, exempt := commitKind(Clean(*in.Commit))
	if exempt || kind == "feat" {
		return res, nil
	}
	files := newFiles(in.Changes)
	if files > themeFiles {
		res.Findings = []model.Finding{shapeFinding(kind, fmt.Sprintf("adds %d new files", files))}
		return res, nil
	}
	added, unread, err := newLines(in.Snapshot, in.Changes)
	if err != nil {
		return model.Result{Status: model.Unmeasured(err.Error())}, nil
	}
	if added > themeLines {
		res.Findings = []model.Finding{shapeFinding(kind, fmt.Sprintf("stages %d new lines", added))}
		return res, nil
	}
	res.Unread = unread
	return res, nil
}

// commitKind answers the type of the subject, empty for a subject without
// one, which is no feat and so keeps to the limits, and whether the law
// leaves the message alone.
func commitKind(message string) (string, bool) {
	_, subject, exempt := subjectOf(lines(message))
	if exempt {
		return "", true
	}
	if m := subjectType.FindStringSubmatch(subject); m != nil {
		return m[1], false
	}
	return "", false
}

// newFiles counts the files the changes add, the new files of section 5.1;
// a modified, renamed or deleted file is no new file.
func newFiles(changes []model.Change) int {
	n := 0
	for _, c := range changes {
		if c.Op == model.ChangeAdd {
			n++
		}
	}
	return n
}

// shapeFinding names the type and what the commit brings over its limit,
// in the words of shell.shape.
func shapeFinding(kind, brings string) model.Finding {
	if kind == "" {
		kind = "typeless"
	}
	text := fmt.Sprintf("a %s commit %s; split it by theme or name it feat (limits %d new files, %d new lines)", kind, brings, themeFiles, themeLines)
	return deny("commit-shape", 0, 0, text)
}
