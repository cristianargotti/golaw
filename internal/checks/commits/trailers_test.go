package commits_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/commits"
	"github.com/cristianargotti/golaw/internal/checks/shell"
	"github.com/cristianargotti/golaw/internal/model"
)

// The trailers of the law, assembled so this file carries none of them.
const (
	coAuthor = "Co-" + "authored-by: Someone <someone@example.com>"
	signOff  = "Sign" + "ed-off-by: Someone <someone@example.com>"
	marker   = "Gener" + "ated with [a tool](https://example.com)"
)

// robot is the emoji a tool writes in front of the marker of a generated
// commit.
const robot = "\U0001F916"

func TestNoTrailersReadsTheTrailersOfTheLaw(t *testing.T) {
	t.Parallel()
	denies(t, commits.NoTrailers(), "trailer", map[string]bool{
		"fix: x\n\n" + coAuthor + "\n":                              true,
		"fix: x\n\nbody\n\n" + signOff + "\n":                       true,
		"fix: x\n\n" + marker + "\n":                                true,
		"fix: x\n\nbody\n" + coAuthor + "\nmore prose\n":            true,
		"fix: x\n\n  " + coAuthor + "\n":                            true,
		"fix: refresh the mocks gener" + "ated with mockery\n":      false,
		"fix: x\n\nthe sign" + "ed-off-by rule stays in the docs\n": false,
		"fix: x\n\nreviewed by someone, thanks\n":                   false,
		"fix: keep the reader\n":                                    false,
		"":                                                          false,

		// A tool writes its marker as a line of its own, with nothing but
		// white space or an emoji in front of it; the same words with a link
		// inside a sentence or after a list bullet are prose.
		"fix: x\n\n" + robot + " " + marker + "\n":                                       true,
		"docs: describe sources gener" + "ated with [protoc](https://example.com)\n":     false,
		"fix: x\n\nthe sources are gener" + "ated with [protoc](https://example.com).\n": false,
		"fix: x\n\n- " + marker + "\n":                                                   false,
	})
}

func TestNoTrailersReadsTheTrailersGitReads(t *testing.T) {
	t.Parallel()
	denies(t, commits.NoTrailers(), "trailer", map[string]bool{
		"fix: x\n\nReviewed-by: a\n":                                                       true,
		"fix: x\n\nNote: the cache stays warm.\n":                                          true,
		"fix: x\n\nKey : value\n":                                                          true,
		"fix: x\n\nRefs: later\n  continued\n":                                             true,
		"fix: x\n\nRefs: 12\nConflicts:\n\tinternal/a.go\n":                                true,
		"fix: x\n\n(cherry picked from commit 0123456)\nReviewed-by: a\none\ntwo\nthree\n": true,
		"fix: x\n\nhttps://example.com/guide\n":                                            false,
		"fix: x\n\nSee the guide.\nhttps://example.com/guide\n":                            false,
		"fix: x\n\nRefs: 12\nsome prose after it\n":                                        false,
		"fix: x\nRefs: 12\n":                                                               false,
		"Refs: 12\n":                                                                       false,
		"fix: x\n\n key: value\n":                                                          false,
		"fix: x\n\n: value\n":                                                              false,
		"fix: x\n\nbody\n---\nRefs: 1\n":                                                   false,
		"fix: x\n\n(cherry picked from commit 0123456)\n":                                  false,
		"fix: x\n\nRefs: 12\nsome prose\n\nclosing prose\n":                                false,
	})
}

func TestNoTrailersNamesEveryTrailerLineOnce(t *testing.T) {
	t.Parallel()
	res := judge(t, commits.NoTrailers(), "fix: x\n\nbody\n\nRefs: 12\n"+signOff+"\n")
	if len(res.Findings) != 2 {
		t.Fatalf("findings = %+v", res.Findings)
	}
	for i, want := range []struct {
		line int
		key  string
	}{{5, "(Refs)"}, {6, "(Sign" + "ed-off-by)"}} {
		f := res.Findings[i]
		if f.Line != want.line || !strings.Contains(f.Message, want.key) || !strings.Contains(f.Message, "no trailer of any kind") {
			t.Errorf("finding %d = %+v, want line %d naming %s", i, f, want.line, want.key)
		}
	}
	led := judge(t, commits.NoTrailers(), "fix: x\n\n"+robot+" "+marker+"\n")
	if len(led.Findings) != 1 || !strings.Contains(led.Findings[0].Message, "(Gener"+"ated with)") {
		t.Errorf("the marker after its emoji = %+v, want it named without the emoji", led.Findings)
	}
}

func TestNoTrailersSkipsTheCommentsOfItsCommentString(t *testing.T) {
	t.Parallel()
	message := "fix: x\n\nRefs: 12\n; a closing note\n"
	semicolon := judgeCommit(t, commits.NoTrailers(), model.Commit{Message: message, Cleanup: model.CleanupWhitespace, CommentChar: ";"})
	hash := judgeCommit(t, commits.NoTrailers(), model.Commit{Message: message, Cleanup: model.CleanupWhitespace})
	if !slices.Equal(codes(semicolon), []string{"trailer"}) || len(hash.Findings) != 0 {
		t.Errorf("comment ; = %v, comment # = %v", codes(semicolon), codes(hash))
	}
	inside := judgeCommit(t, commits.NoTrailers(), model.Commit{Message: "fix: x\n\nRefs: 12\n# kept\nReviewed-by: a\n", Cleanup: model.CleanupWhitespace})
	if !slices.Equal(codes(inside), []string{"trailer", "trailer"}) {
		t.Errorf("a comment inside the block = %v", codes(inside))
	}
}

func TestTrailersAgreeWithTheShellFamily(t *testing.T) {
	t.Parallel()
	agree(t, shell.Trailers(), commits.NoTrailers(), "trailer", map[message]bool{
		{"fix: x", coAuthor}:                      true,
		{"fix: x", signOff}:                       true,
		{"fix: x", marker}:                        true,
		{"fix: x", robot + " " + marker}:          true,
		{"fix: x", "reviewed by someone, thanks"}: false,
		{"fix: keep the reader", ""}:              false,
	})
}

// The shell family reads the trailer phrases anywhere in a message given on
// the command line and no other key; the commit rung reads what git records,
// so it leaves a phrase in prose alone and refuses a trailer of any key.
func TestTrailersGoBeyondTheShellFamilyByDesign(t *testing.T) {
	t.Parallel()
	for _, prose := range []message{
		{"fix: refresh the mocks gener" + "ated with mockery", ""},
		{"docs: describe sources gener" + "ated with [protoc](https://example.com)", ""},
	} {
		if !shellFinds(t, shell.Trailers(), "trailer", prose) || commitFinds(t, commits.NoTrailers(), "trailer", prose) {
			t.Errorf("%q: a trailer phrase in prose: the shell family denies it, the commit rung does not", prose.subject)
		}
	}
	review := message{"fix: x", "Reviewed-by: a"}
	if shellFinds(t, shell.Trailers(), "trailer", review) || !commitFinds(t, commits.NoTrailers(), "trailer", review) {
		t.Error("a trailer of another key: the commit rung denies it, the shell family does not see it")
	}
}
