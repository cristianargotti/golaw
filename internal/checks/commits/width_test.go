package commits_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/commits"
	"github.com/cristianargotti/golaw/internal/checks/shell"
	"github.com/cristianargotti/golaw/internal/model"
)

// Body lines one character over the width and exactly at it, in ASCII and
// in a two-byte rune built from its code point.
var (
	overWidth  = strings.Repeat("a", 101)
	atWidth    = strings.Repeat("a", 100)
	wideAt     = strings.Repeat(string(rune(0xe9)), 100)
	wideOver   = strings.Repeat(string(rune(0xe9)), 101)
	commentFor = "# " + overWidth
)

func TestBodyWidthReadsEveryBodyLine(t *testing.T) {
	t.Parallel()
	denies(t, commits.BodyWidth(), "body-width", map[string]bool{
		"fix: x\n\n" + overWidth + "\n":      true,
		"fix: x\n\n" + atWidth + "\n":        false,
		"fix: " + overWidth + "\n":           false,
		"fix: x\n\n" + commentFor + "\n":     false,
		"fix: x\n\n" + wideAt + "\n":         false,
		"fix: x\n\n" + wideOver + "\n":       true,
		"\n\nfix: x\n\nshort\n" + overWidth:  true,
		"fix: x\n#" + cut + overWidth + "\n": false,
		"":                                   false,
	})
	verbatim := judgeCommit(t, commits.BodyWidth(), model.Commit{Message: "fix: x\r\n\r\n" + atWidth + "\r\n", Cleanup: model.CleanupVerbatim})
	if len(verbatim.Findings) != 0 {
		t.Errorf("a carriage return is no character: %+v", verbatim.Findings)
	}
}

func TestBodyWidthNamesEveryLongLine(t *testing.T) {
	t.Parallel()
	res := judge(t, commits.BodyWidth(), "fix: x\n\n"+overWidth+"\n"+atWidth+"\n"+overWidth+"a\n")
	if len(res.Findings) != 2 {
		t.Fatalf("findings = %+v", res.Findings)
	}
	for i, want := range []struct {
		line  int
		width string
	}{{3, "101 characters"}, {5, "102 characters"}} {
		f := res.Findings[i]
		if f.Line != want.line || f.Column != 101 || !strings.Contains(f.Message, want.width) || !strings.Contains(f.Message, "wrap the body") {
			t.Errorf("finding %d = %+v, want line %d holding %s", i, f, want.line, want.width)
		}
	}
}

func TestBodyWidthAgreesWithTheShellFamily(t *testing.T) {
	t.Parallel()
	agree(t, shell.BodyWidth(), commits.BodyWidth(), "body-width", map[message]bool{
		{"fix: x", overWidth}:     true,
		{"fix: x", wideOver}:      true,
		{"fix: x", atWidth}:       false,
		{"fix: x", wideAt}:        false,
		{"fix: " + overWidth, ""}: false,
	})
}
