package commits_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/commits"
	"github.com/cristianargotti/golaw/internal/checks/shell"
	"github.com/cristianargotti/golaw/internal/model"
)

// The prohibited runes, built from their code points so this file carries
// none.
var (
	emDash = string(rune(0x2014))
	enDash = string(rune(0x2013))
)

func TestNoDashesReadsTheMessageGitRecords(t *testing.T) {
	t.Parallel()
	denies(t, commits.NoDashes(), "em-dash", map[string]bool{
		"fix: a " + emDash + " b\n":                   true,
		"fix: a - b\n\nbody " + emDash + " here\n":    true,
		"fix: a, b\n\nbody: here\n":                   false,
		"fix: a\n\n# a comment " + emDash + " goes\n": false,
		"fix: a\n#" + cut + "diff " + emDash + " x\n": false,
		"fix: a " + emDash + emDash + " b\n":          true,
		"fix: pages 1" + enDash + "3\n\nbody, here\n": false,
	})
	denies(t, commits.NoDashes(), "en-dash", map[string]bool{
		"fix: pages 1" + enDash + "3\n": true,
		"fix: pages 1-3\n":              false,
	})
	kept := judgeCommit(t, commits.NoDashes(), model.Commit{Message: "fix: a\n\n# a comment " + emDash + " stays\n", Cleanup: model.CleanupWhitespace})
	if !slices.Equal(codes(kept), []string{"em-dash"}) {
		t.Errorf("a comment git keeps: %v", codes(kept))
	}
}

func TestNoDashesNamesTheLineAndColumnOfEachKind(t *testing.T) {
	t.Parallel()
	res := judge(t, commits.NoDashes(), "fix: a "+emDash+" b\n\nfirst "+enDash+" then "+emDash+"\n")
	want := []struct {
		code         string
		line, column int
	}{{"em-dash", 1, 8}, {"em-dash", 3, 14}, {"en-dash", 3, 7}}
	if len(res.Findings) != len(want) {
		t.Fatalf("findings = %+v", res.Findings)
	}
	for i, w := range want {
		f := res.Findings[i]
		if f.Code != w.code || f.Line != w.line || f.Column != w.column {
			t.Errorf("finding %d = %+v, want %+v", i, f, w)
		}
	}
}

func TestDashesAgreeWithTheShellFamily(t *testing.T) {
	t.Parallel()
	agree(t, shell.Dashes(), commits.NoDashes(), "em-dash", map[message]bool{
		{"fix: a " + emDash + " b", ""}: true,
		{"fix: a", "body " + emDash}:    true,
		{"fix: a", "# note " + emDash}:  true,
		{"fix: a - b", "body, plain"}:   false,
		{"fix: keep the reader", ""}:    false,
	})
	agree(t, shell.Dashes(), commits.NoDashes(), "en-dash", map[message]bool{
		{"fix: pages 1" + enDash + "3", ""}: true,
		{"fix: pages 1-3", ""}:              false,
	})
}
