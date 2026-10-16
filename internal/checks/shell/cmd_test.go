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

// TestCmdSeesTheCommandsOfItsControlBlocks judges the command an if, a for
// or an else of cmd runs: the condition of an if (exist, defined,
// errorlevel, cmdextversion, a comparison, with /i and not), the clause of
// a for up to its do and an else after a closing parenthesis are no part
// of the command, a parenthesis right after them opens a block, the set of
// a for holds parentheses of its own, and the delimiters cmd skips before
// a command are no part of its name; an else that follows no block and an
// if that is an argument of another command run nothing.
func TestCmdSeesTheCommandsOfItsControlBlocks(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		tool, command string
		want          string
	}{
		{"Bash", `cmd /c 'if exist marker (git commit --no-verify -m "fix: x")'`, "no-verify"},
		{"PowerShell", `cmd /c 'if exist marker (git commit --no-verify -m "fix: x")'`, "no-verify"},
		{"Bash", `cmd /c 'if exist marker git commit --no-verify -m x'`, "no-verify"},
		{"Bash", `cmd /c 'if not exist marker (git commit --no-verify -m x)'`, "no-verify"},
		{"Bash", `cmd /c 'if exist marker (echo y) else (git commit --no-verify -m x)'`, "no-verify"},
		{"Bash", `cmd /c 'if exist marker (echo y) else(git commit --no-verify -m x)'`, "no-verify"},
		{"Bash", `cmd /c 'for %i in (1) do (git commit --no-verify -m x)'`, "no-verify"},
		{"Bash", `cmd /c 'for /l %i in (1,1,2) do(git commit --no-verify -m x)'`, "no-verify"},
		{"Bash", `cmd /c '(for %i in (1) do git commit --no-verify -m x)'`, "no-verify"},
		{"Bash", `cmd /c 'if "a b"=="a b" git commit --no-verify -m x'`, "no-verify"},
		{"Bash", `cmd /c 'if /i a equ A (git commit --no-verify -m x)'`, "no-verify"},
		{"Bash", `cmd /c 'if a == a git commit --no-verify -m x'`, "no-verify"},
		{"Bash", `cmd /c 'if a== a git commit --no-verify -m x'`, "no-verify"},
		{"Bash", `cmd /c 'if a ==a git commit --no-verify -m x'`, "no-verify"},
		{"Bash", `cmd /c 'if errorlevel 1 if defined X @git commit --no-verify -m x'`, "no-verify"},
		{"Bash", `cmd /c 'if cmdextversion 2 git commit --no-verify -m x'`, "no-verify"},
		{"Bash", `cmd /c ';git commit --no-verify -m x'`, "no-verify"},
		{"Bash", `cmd /c 'echo a & ,=git commit --no-verify -m x'`, "no-verify"},
		{"Bash", `cmd /c 'if exist marker (git commit -m "fix: x")'`, ""},
		{"Bash", `cmd /c 'if exist marker git commit -m "--no-verify"'`, ""},
		{"Bash", `cmd /c 'if exist marker (echo --no-verify)'`, ""},
		{"Bash", `cmd /c 'if 1==1 echo a else git commit --no-verify'`, ""},
		{"Bash", `cmd /c 'echo if exist x git commit --no-verify'`, ""},
		{"Bash", `cmd /c 'for %i in (git commit --no-verify) do echo %i'`, ""},
		{"Bash", `cmd /c 'if exist'`, ""},
		{"Bash", `cmd /c 'if a b c git commit --no-verify -m x'`, ""},
		{"Bash", `cmd /c 'for %i in (do) do git commit --no-verify -m x'`, "no-verify"},
		{"Bash", `cmd /c 'for %i in (a(do)) do git commit --no-verify -m x'`, "no-verify"},
	} {
		got := judgeTool(t, shell.NoVerify(), tc.tool, tc.command)
		if tc.want == "" && len(got) != 0 || tc.want != "" && (len(got) != 1 || got[0] != tc.want) {
			t.Errorf("%s %q: codes %v, want %q", tc.tool, tc.command, got, tc.want)
		}
	}
}

// TestCmdJudgesTheNamesOfItsControlPrefixes judges the names a control
// prefix of cmd holds, which the command it runs reads: the items of the
// set of a for, spaced or glued to its parentheses, quoted or apart by a
// comma, the root of a for /r and the file of an if exist; a set of plain
// names and the file of a plain if pass.
func TestCmdJudgesTheNamesOfItsControlPrefixes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		tool, command string
		want          string
	}{
		{"Bash", `cmd /c 'for /f %i in ( .env ) do echo %i'`, "secret-token"},
		{"PowerShell", `cmd /c 'for /f %i in ( .env ) do echo %i'`, "secret-token"},
		{"Bash", `cmd /c 'for %i in ( secrets\token.txt ) do type %i'`, "secret-token"},
		{"Bash", `cmd /c 'for /f %i in (.env) do echo %i'`, "secret-token"},
		{"Bash", `cmd /c 'for /f "usebackq" %i in (".env") do echo %i'`, "secret-token"},
		{"Bash", `cmd /c 'for %i in (a.txt,secrets\token.txt) do type %i'`, "secret-token"},
		{"Bash", `cmd /c 'for %i in (a.txt;.env) do type %i'`, "secret-token"},
		{"Bash", `cmd /c 'for /r deploy\secrets\db %i in (*) do type %i'`, "secret-token"},
		{"Bash", `cmd /c 'if exist .env (echo y)'`, "secret-token"},
		{"Bash", `cmd /c 'if exist .env git status'`, "secret-token"},
		{"Bash", `cmd /c '(for %i in (.env) do type %i)'`, "secret-token"},
		{"Bash", `cmd /c 'for %i in (a.txt b.txt) do type %i'`, ""},
		{"Bash", `cmd /c 'for /l %i in (1,1,3) do echo %i'`, ""},
		{"Bash", `cmd /c 'if exist marker (echo y)'`, ""},
		{"Bash", `cmd /c 'if exist marker type notes.md'`, ""},
	} {
		got := judgeTool(t, shell.SecretTokens(), tc.tool, tc.command)
		if tc.want == "" && len(got) != 0 || tc.want != "" && (len(got) != 1 || got[0] != tc.want) {
			t.Errorf("%s %q: codes %v, want %q", tc.tool, tc.command, got, tc.want)
		}
	}
}
