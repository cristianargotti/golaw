// Package text holds the checks that read a file as lines of text: the dash
// law, the public boundary terms, the comment language and the code hygiene
// markers. Every check reads bytes only through the snapshot, so a candidate
// that does not exist on disk yet is judged exactly like a committed file.
package text

import (
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

// anyText keeps every file a check may read as text.
func anyText(f model.File) bool { return f.Class.IsText() }

// walk reads every target that accept keeps and hands each line to visit
// with its 1-based number. It returns how many files it read; a failed read
// ends the walk with the error so the check answers Unmeasured, never clean.
func walk(in model.Input, accept keep, visit func(path string, number int, line string)) (int, error) {
	files := 0
	for _, f := range in.Targets {
		if !accept(f) {
			continue
		}
		content, err := in.Snapshot.Read(f.Path)
		if err != nil {
			return files, fmt.Errorf("read %s: %w", f.Path, err)
		}
		files++
		for i, line := range strings.Split(string(content), "\n") {
			visit(f.Path, i+1, line)
		}
	}
	return files, nil
}

// result turns a walk into the check result: Unmeasured on a read failure,
// Measured with the file count otherwise.
func result(files int, err error, findings []model.Finding) model.Result {
	if err != nil {
		return model.Result{Status: model.Unmeasured(err.Error())}
	}
	return model.Result{Status: model.Measured(), Scope: fmt.Sprintf("%d files", files), Findings: findings}
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
