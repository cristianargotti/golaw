package shell_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/shell"
	"github.com/cristianargotti/golaw/internal/model"
)

// TestCmdReadsItsScriptTheWayCmdAndTheRuntimeDo judges a script run by
// cmd /c the way it reaches the program: cmd splits the commands where its
// own quotes, which a backslash never escapes, leave a separator outside,
// a caret makes the next rune literal, a semicolon separates nothing, a
// parenthesis opens or closes a block, a leading at sign is dropped and a
// script that starts with a quote loses it and the last one; the program
// then splits its arguments by the Microsoft rules, where a run of
// backslashes before a double quote keeps one per pair and an odd one
// makes the quote literal, and a single quote is an ordinary rune.
func TestCmdReadsItsScriptTheWayCmdAndTheRuntimeDo(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		check         model.Check
		tool, command string
		want          string
	}{
		{shell.NoVerify(), "Bash", `cmd /c 'git commit -m "fix: \"" --no-verify'`, "no-verify"},
		{shell.NoVerify(), "PowerShell", `cmd /c 'git commit -m "fix: \"" --no-verify'`, "no-verify"},
		{shell.NoVerify(), "Bash", `cmd /c 'git commit -m "fix: \" --no-verify"'`, ""},
		{shell.NoVerify(), "Bash", `cmd /c 'git commit -m "x\\\\" --no-verify'`, "no-verify"},
		{shell.NoVerify(), "Bash", `cmd /c 'git commit -m "x\\\" --no-verify"'`, ""},
		{shell.NoVerify(), "Bash", `cmd /c 'git commit -m "x\" & git commit --no-verify -m y"'`, "no-verify"},
		{shell.NoVerify(), "Bash", `cmd /c 'git commit -m "a & git commit --no-verify"'`, ""},
		{shell.NoVerify(), "Bash", `cmd /c "git commit -m 'a --no-verify b'"`, "no-verify"},
		{shell.NoVerify(), "Bash", `cmd /c 'git commit -m x ^& --no-verify'`, "no-verify"},
		{shell.NoVerify(), "Bash", `cmd /c 'git commit -m x; --no-verify'`, "no-verify"},
		{shell.NoVerify(), "Bash", `cmd /c '"git commit --no-verify -m x"'`, "no-verify"},
		{shell.NoVerify(), "Bash", `cmd /c '"C:\Program Files\Git\cmd\git.exe" commit --no-verify -m x'`, "no-verify"},
		{shell.NoVerify(), "Bash", `cmd /c '(git commit --no-verify -m x)'`, "no-verify"},
		{shell.NoVerify(), "Bash", `cmd /c 'git commit -m (x) --no-verify'`, "no-verify"},
		{shell.NoVerify(), "Bash", `cmd /c '@git commit --no-verify -m x'`, "no-verify"},
		{shell.NoVerify(), "Bash", `cmd /c 'git commit -m "a "" --no-verify"'`, "no-verify"},
		{shell.NoVerify(), "Bash", `cmd /c '"git commit --no-verify -m x'`, "no-verify"},
		{shell.NoVerify(), "Bash", `cmd /c '"git" commit -m "x" --no-verify'`, ""},
		{shell.NoVerify(), "Bash", "cmd /c 'git commit -m x ^\n --no-verify'", "no-verify"},
		{shell.NoVerify(), "Bash", "cmd /c 'git status\r\ngit commit --no-verify -m y'", "no-verify"},
		{shell.NoVerify(), "Bash", "cmd /c 'git commit -m x\necho --no-verify'", ""},
		{shell.NoVerify(), "Bash", `cmd /c 'git commit -m x & echo --no-verify'`, ""},
		{shell.NoVerify(), "Bash", `cmd /c "git commit -m x"`, ""},
		{shell.NoVerify(), "Bash", `cmd /c "C:\Git\cmd\git.exe commit --no-verify -m x"`, "no-verify"},
		{shell.SecretTokens(), "Bash", `cmd /c 'type "C:\work\secrets\db.yaml"'`, "secret-token"},
		{shell.SecretTokens(), "Bash", `cmd /c 'type "C:\work\docs\notes.md"'`, ""},
	} {
		got := judgeTool(t, tc.check, tc.tool, tc.command)
		if tc.want == "" && len(got) != 0 || tc.want != "" && (len(got) != 1 || got[0] != tc.want) {
			t.Errorf("%s %q: codes %v, want %q", tc.tool, tc.command, got, tc.want)
		}
	}
}
