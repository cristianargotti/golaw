package shell_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/shell"
)

// TestCommentsAreNoCommands reads a comment the way the shell does: a #
// that starts a word outside quotes makes the rest of its line text that no
// rule reads and the shell never expands, so a quote in it opens nothing
// and a << in it opens no here-document, a comment inside a substitution
// included, while a # inside a word is a rune of the word; in PowerShell a
// block comment runs from <# to #>, over lines too.
func TestCommentsAreNoCommands(t *testing.T) {
	t.Parallel()
	expectCodes(t, shell.NoVerify(), []toolCase{
		{"Bash", "# don't skip\ngit commit --no-verify -m x", "no-verify"},
		{"Bash", "git status # don't\ngit commit --no-verify -m x", "no-verify"},
		{"Bash", "# see <<x\ngit commit --no-verify -m x", "no-verify"},
		{"Bash", "echo a#b; git commit --no-verify -m x", "no-verify"},
		{"Bash", "echo $#; git commit --no-verify -m x", "no-verify"},
		{"Bash", "echo ${#x}; git commit --no-verify -m x", "no-verify"},
		{"Bash", "echo \"$(echo a # )\n git commit --no-verify -m x)\"", "no-verify"},
		{"PowerShell", "# don't\ngit commit --no-verify -m x", "no-verify"},
		{"PowerShell", "<# note #> git commit --no-verify -m x", "no-verify"},
		{"PowerShell", "<# don't\n#>\ngit commit --no-verify -m x", "no-verify"},
		{"PowerShell", "git commit <# x #> --no-verify -m x", "no-verify"},
		{"Bash", "# never run `git commit --no-verify`\ngo test ./...", ""},
		{"Bash", "go test ./... # not git commit --no-verify", ""},
		{"Bash", "# git commit --no-verify -m x", ""},
		{"PowerShell", "# the \"$(git commit --no-verify -m x)\" form\ngo test ./...", ""},
		{"PowerShell", "<# git commit --no-verify -m x #> git status", ""},
		{"PowerShell", "<# a\ngit commit --no-verify -m x\n#> git status", ""},
	})
	expectCodes(t, shell.SecretTokens(), []toolCase{
		{"Bash", "# it's\ncat .env", "secret-token"},
		{"Bash", "# copy `.env` first\ngo test ./...", ""},
		{"Bash", "ls # then cat .env", ""},
	})
}
