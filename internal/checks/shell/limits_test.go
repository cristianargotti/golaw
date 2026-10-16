package shell_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/shell"
)

// TestNoVerifyReadsTheLineAsWritten pins the limit docs/security.md states
// next to the promise that a commit with the hook bypass flag is denied:
// the hook reads the command line as written, so a command line built
// while it runs passes, because reading it would need a full interpreter:
// the value of a variable eval runs, a program named by the output of a
// command, the input xargs reads, the lines a for /f hands its body and a
// variable cmd expands; and the blocks of a PowerShell statement after its
// first are read as one command with the words before them.
func TestNoVerifyReadsTheLineAsWritten(t *testing.T) {
	t.Parallel()
	expectCodes(t, shell.NoVerify(), []toolCase{
		{"Bash", `c='git commit --no-verify -m x'; eval "$c"`, ""},
		{"Bash", "$(echo git) commit --no-verify -m x", ""},
		{"Bash", "echo --no-verify | xargs git commit -m x", ""},
		{"Bash", `cmd /c "for /f %i in ('echo --no-verify') do git commit %i -m x"`, ""},
		{"Bash", `cmd /c "set G=git & call %G% commit --no-verify -m x"`, ""},
		{"PowerShell", "if ($a) { echo y } else { git commit --no-verify -m x }", ""},
		{"Bash", `eval "git commit --no-verify -m x"`, "no-verify"},
		{"Bash", "echo x | xargs git commit --no-verify -m x", "no-verify"},
		{"PowerShell", "if ($a) { git commit --no-verify -m x } else { echo y }", "no-verify"},
	})
}
