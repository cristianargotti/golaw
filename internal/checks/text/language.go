package text

import (
	"context"
	"regexp"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// The comment grammar and the two signs of a comment that is not English: a
// Spanish or Portuguese function word, or two accented letters outside
// quotes (one alone is a proper noun or a quoted term). The two-letter
// articles stay out of the function words, because el and la are as often
// Go names an English comment talks about. The accented letters are
// spelled by code point so this file carries none.
var (
	commentLine  = regexp.MustCompile(`^\s*(?://|/\*|\*)\s?(.*)$`)
	quotedText   = regexp.MustCompile("\"[^\"]*\"|'[^']*'|`[^`]*`")
	functionWord = regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}_])(?:los|las|una|para|porque|n\x{e3}o|tamb\x{e9}m|isso|esto|aqui|ent\x{e3}o|hasta|pero|quando|cuando|j\x{e1}|est\x{e1}|deve|debe|cada|ainda|todav\x{ed}a|sino|sen\x{e3}o)(?:$|[^\p{L}\p{N}_])`)
	accentedRune = regexp.MustCompile(`[\x{e1}\x{e9}\x{ed}\x{f3}\x{fa}\x{e3}\x{f5}\x{e7}\x{f1}\x{c1}\x{c9}\x{cd}\x{d3}\x{da}]`)
)

// goFile keeps hand-written Go files, an unreadable one included (anyText);
// generated code has no author.
func goFile(f model.File) bool {
	return anyText(f) && f.Class != model.ClassGenerated && strings.HasSuffix(f.Path, ".go")
}

type commentLanguage struct{}

// CommentLanguage is text.comment-language: every comment of a Go file is
// English; the human channel strings are not comments and are not read.
func CommentLanguage() model.Check { return commentLanguage{} }

func (commentLanguage) Spec() model.Spec { return spec("text.comment-language") }

// Run reports every comment line that reads as Spanish or Portuguese.
func (commentLanguage) Run(_ context.Context, in model.Input) (model.Result, error) {
	var findings []model.Finding
	files, err := walk(in, goFile, func(path string, number int, line string) {
		loc := commentLine.FindStringSubmatchIndex(line)
		if len(loc) == 0 || english(line[loc[2]:loc[3]]) {
			return
		}
		message := "code comment not in English; comments are English and only for the why"
		findings = append(findings, finding(path, number, column(line, loc[2]), "comment-language", message))
	})
	return result(files, err, findings), nil
}

// english reports whether a comment reads as English: no Spanish or
// Portuguese function word and fewer than two accented letters once the
// quoted segments are removed.
func english(comment string) bool {
	bare := quotedText.ReplaceAllString(comment, "")
	return !functionWord.MatchString(bare) && len(accentedRune.FindAllStringIndex(bare, -1)) < 2
}
