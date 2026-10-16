package shell

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cristianargotti/golaw/internal/model"
)

// bodyWidth is the widest a commit body line may be, in characters.
const bodyWidth = 100

// BodyWidth is shell.body-width: every body line of a commit message given
// on the command line fits in 100 characters; the subject is the commit
// rung's business.
func BodyWidth() model.Check {
	return commandRule{spec: spec("shell.body-width"), judge: judgeBodyWidth}
}

func judgeBodyWidth(_ model.Input, segs []segment) ([]model.Finding, error) {
	for _, call := range gitCalls(segs) {
		if !isCommit(call) {
			continue
		}
		for _, line := range strings.Split(messageOf(call), "\n")[1:] {
			if utf8.RuneCountInString(line) > bodyWidth {
				message := fmt.Sprintf("a commit body line exceeds %d characters; wrap the body", bodyWidth)
				return []model.Finding{deny("body-width", message)}, nil
			}
		}
	}
	return nil, nil
}
