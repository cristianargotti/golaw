// Package text holds the checks that read a file as lines of text: the dash
// law, the public boundary terms, the comment language and the code hygiene
// markers. Every check reads bytes only through the snapshot, so a candidate
// that does not exist on disk yet is judged exactly like a committed file.
package text

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cristianargotti/golaw/internal/model"
)

// rule is the rule reference of the family until docs/rules.md ships.
const rule = "rules/4.1@1"

// Checks lists every check of the family in report order.
func Checks() []model.Check {
	return []model.Check{Dashes(), TraceTerms(), ToolTerms(), CommentLanguage(), TaskMarker(), BareNolint(), NolintNilnil(), DebugPrint()}
}

// spec builds the specification every text check shares: file scope, the
// core owner, the family rule and the proof key equal to the id.
func spec(id string) model.Spec {
	return model.Spec{ID: id, Rule: rule, Scope: model.ScopeFile, Owner: model.OwnerCore, Proof: id}
}

// keep decides whether a check reads a target.
type keep func(model.File) bool

// anyText keeps every file a check may read as text, and every file of
// the unreadable class: that class hides what the bytes would be, so the
// check reads the file through the snapshot, the read fails and the walk
// answers Unmeasured naming the path instead of clean (section 5.2).
func anyText(f model.File) bool { return f.Class.IsText() || f.Class == model.ClassUnreadable }

// walked is what a walk read: how many files, and the files it left unread
// because they are over the read bound.
type walked struct {
	files  int
	unread []string
}

// walk reads every target that accept keeps and hands each line to visit
// with its 1-based number. A target over the read bound is left unread and
// named, and the walk goes on; any other failed read ends the walk with
// the error so the check answers Unmeasured, never clean.
func walk(in model.Input, accept keep, visit func(path string, number int, line string)) (walked, error) {
	var w walked
	for _, f := range in.Targets {
		if !accept(f) {
			continue
		}
		content, err := in.Snapshot.Read(f.Path)
		if errors.Is(err, model.ErrTooLarge) {
			w.unread = append(w.unread, f.Path)
			continue
		}
		if err != nil {
			return w, fmt.Errorf("read %s: %w", f.Path, err)
		}
		w.files++
		for i, line := range strings.Split(string(content), "\n") {
			visit(f.Path, i+1, line)
		}
	}
	return w, nil
}

// result turns a walk into the check result: Unmeasured on a read failure,
// Measured with the file count and the files left unread otherwise.
func result(w walked, err error, findings []model.Finding) model.Result {
	if err != nil {
		return model.Result{Status: model.Unmeasured(err.Error())}
	}
	return model.Result{Status: model.Measured(), Scope: fmt.Sprintf("%d files", w.files), Findings: findings, Unread: w.unread}
}

// finding builds a denial at a line and column; the engine fills the check,
// the rule and the fingerprint.
func finding(path string, line, column int, code, message string) model.Finding {
	return model.Finding{Path: path, Line: line, Column: column, Code: code, Message: message, Severity: model.SeverityDeny}
}

// column converts the byte offset of a match into a 1-based rune column.
func column(line string, offset int) int {
	return utf8.RuneCountInString(line[:offset]) + 1
}
