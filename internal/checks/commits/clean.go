package commits

import (
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// cutLine is what git writes after the comment string and a space above
// the diff of git commit --verbose and in the scissors mode.
const cutLine = "------------------------ >8 ------------------------"

// gitSpace is what git trims from the end of a line: its own isspace,
// which leaves the vertical tab and the form feed in place.
const gitSpace = " \t\r\n"

// Clean is the message git records: the cleanup of c.Cleanup applied to the
// message the hook read, as git applies it after the commit-msg hook. The
// scissors line (the comment character followed by
// " ------------------------ >8 ------------------------") and everything
// below it are dropped in every mode, because git writes it only for
// --verbose or the scissors mode and drops what follows in both; strip also
// drops every line that starts with the comment character; strip,
// whitespace and scissors drop trailing whitespace, leading and trailing
// blank lines and repeated blank lines; verbatim keeps the rest as it is.
func Clean(c model.Commit) string {
	comment := commentOf(c)
	message := cutScissors(c.Message, comment)
	switch c.Cleanup {
	case model.CleanupVerbatim:
		return message
	case model.CleanupWhitespace, model.CleanupScissors:
		return stripSpace(message, "")
	default:
		return stripSpace(message, comment)
	}
}

// commentOf answers the default of git for a commit that names no comment
// string, because git cleaned that message with the default.
func commentOf(c model.Commit) string {
	if c.CommentChar == "" {
		return model.DefaultCommentChar
	}
	return c.CommentChar
}

// cutScissors drops the scissors line and everything below it, where the
// line starts the message or follows a line break, as git finds it.
func cutScissors(message, comment string) string {
	line := comment + " " + cutLine + "\n"
	if strings.HasPrefix(message, line) {
		return ""
	}
	if i := strings.Index(message, "\n"+line); i >= 0 {
		return message[:i+1]
	}
	return message
}

// stripSpace is the stripspace of git: every line loses its trailing white
// space, a line that starts with the comment string goes when one is
// given, a run of blank lines becomes one, the blank lines at both ends go
// and every kept line ends with a line break.
func stripSpace(message, comment string) string {
	var kept []string
	blank := false
	for line := range strings.SplitSeq(message, "\n") {
		if comment != "" && strings.HasPrefix(line, comment) {
			continue
		}
		line = strings.TrimRight(line, gitSpace)
		if line == "" {
			blank = true
			continue
		}
		if blank && len(kept) > 0 {
			kept = append(kept, "")
		}
		blank = false
		kept = append(kept, line)
	}
	if len(kept) == 0 {
		return ""
	}
	return strings.Join(kept, "\n") + "\n"
}
