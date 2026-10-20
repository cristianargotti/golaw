package commits

import (
	"fmt"
	"regexp"

	"github.com/cristianargotti/golaw/internal/model"
)

// conventional reads a subject the way the standard writes it: a type of
// the list, an optional scope of letters, digits, dots, underscores,
// slashes and hyphens, an optional bang for a breaking change, a colon, one
// space and a description that starts with a lower case letter.
var conventional = regexp.MustCompile(`^(?:feat|fix|chore|docs|refactor|test|build|ci|perf|style|revert)(?:\([A-Za-z0-9._/-]+\))?!?: [a-z]`)

// Conventional is commit.conventional: the subject is type(scope)!:
// description. A message git writes itself is never a finding, and an
// autosquash prefix is read in front of a conventional subject.
func Conventional() model.Check {
	return messageRule{spec: spec("commit.conventional"), judge: judgeConventional}
}

func judgeConventional(message, _ string) []model.Finding {
	at, subject, exempt := subjectOf(lines(message))
	if exempt || conventional.MatchString(subject) {
		return nil
	}
	text := fmt.Sprintf("the subject %q is not a conventional commit: write type(scope): description, the type one of "+
		"feat, fix, chore, docs, refactor, test, build, ci, perf, style or revert, the scope optional and the "+
		"description starting with a lower case letter", subject)
	return []model.Finding{deny("conventional-subject", at+1, 1, text)}
}
