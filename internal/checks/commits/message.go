package commits

import (
	"regexp"
	"strings"
)

// gitWritten reads a subject git writes itself: a merge of branches, tags,
// commits or a pull request, a revert and the revert of a revert. The laws
// leave such a message as git wrote it.
var gitWritten = regexp.MustCompile(`^(?:Merge (?:branch|branches|remote-tracking branch|remote-tracking branches|tag|tags|commit|commits) '|Merge pull request #\d+ from |Revert "|Reapply ")`)

// autosquash is the prefix git commit --fixup, --squash and the amend form
// of --fixup write in front of the subject of the commit they fix.
var autosquash = regexp.MustCompile(`^(?:(?:fixup|squash|amend)! )+`)

// lines splits a message into its lines; the line break that ends the last
// line starts no empty line, and an empty message has none.
func lines(message string) []string {
	if message == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(message, "\n"), "\n")
}

// subjectAt answers the index of the first line that holds more than white
// space, the line git reads as the subject, or -1 when none does.
func subjectAt(ls []string) int {
	for i, line := range ls {
		if strings.TrimSpace(line) != "" {
			return i
		}
	}
	return -1
}

// subjectOf answers the subject the laws read, the first line without its
// autosquash prefixes, with its index, and whether the laws leave the
// message alone: a message git writes itself, or an empty one, which git
// refuses by itself.
func subjectOf(ls []string) (int, string, bool) {
	at := subjectAt(ls)
	if at < 0 {
		return at, "", true
	}
	subject := autosquash.ReplaceAllString(ls[at], "")
	return at, subject, gitWritten.MatchString(subject)
}
