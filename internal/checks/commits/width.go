package commits

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cristianargotti/golaw/internal/model"
)

// bodyWidth is the widest a body line of the message may be, in
// characters, the width shell.body-width holds a command line message to.
const bodyWidth = 100

// BodyWidth is commit.body-width: every line of the body git records, the
// lines after the subject, holds at most 100 characters. A carriage return
// a verbatim message keeps is no character.
func BodyWidth() model.Check {
	return messageRule{spec: spec("commit.body-width"), judge: judgeWidth}
}

func judgeWidth(message, _ string) []model.Finding {
	ls := lines(message)
	var out []model.Finding
	for i := subjectAt(ls) + 1; i < len(ls); i++ {
		width := utf8.RuneCountInString(strings.TrimSuffix(ls[i], "\r"))
		if width > bodyWidth {
			text := fmt.Sprintf("line %d of the commit body holds %d characters, over %d; wrap the body", i+1, width, bodyWidth)
			out = append(out, deny("body-width", i+1, bodyWidth+1, text))
		}
	}
	return out
}
