package shell_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/shell"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// judgeTool runs one check over a command sent through a shell tool.
func judgeTool(t *testing.T, c model.Check, tool, command string) []string {
	t.Helper()
	in := input(command, proof.Policy(), proof.NewTree(nil))
	in.Event.Tool, in.Event.Shell.Tool = tool, tool
	res, err := c.Run(t.Context(), in)
	if err != nil || res.Status.Kind != model.StatusMeasured {
		t.Fatalf("%s %q: %+v, %v", tool, command, res, err)
	}
	return codes(res)
}

// TestPowerShellKeepsItsBackslashes reads a PowerShell command the way
// PowerShell does: the backslash is the path separator and the backtick
// the escape, so a native path to git still names git and a native path
// to secret material still names it; the same words through Bash keep
// the POSIX escapes, and a script that Bash hands to pwsh in double quotes
// keeps the backslashes Bash leaves there.
func TestPowerShellKeepsItsBackslashes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		check         model.Check
		tool, command string
		want          string
	}{
		{shell.NoVerify(), "PowerShell", `C:\Git\cmd\git.exe commit --no-verify -m 'fix: safe'`, "no-verify"},
		{shell.NoVerify(), "PowerShell", `& "C:\Program Files\Git\cmd\git.exe" commit -n -m 'fix: safe'`, "no-verify"},
		{shell.NoVerify(), "PowerShell", `& "C:\Program Files\Git\cmd\git.exe" commit -m 'fix: safe'`, ""},
		{shell.NoVerify(), "PowerShell", "git commit -m \"fix: say `\"no`\"\" --no-verify", "no-verify"},
		{shell.NoVerify(), "Bash", `pwsh -Command "C:\Git\cmd\git.exe commit --no-verify -m x"`, "no-verify"},
		{shell.NoVerify(), "Bash", `cmd /c "C:\Git\cmd\git.exe commit --no-verify -m x"`, "no-verify"},
		{shell.Push(), "PowerShell", `C:\Git\cmd\git.exe push origin main`, "push"},
		{shell.Push(), "PowerShell", `C:\Git\cmd\git.exe fetch origin`, ""},
		{shell.SecretTokens(), "PowerShell", `Get-Content C:\work\secrets\db.yaml`, "secret-token"},
		{shell.SecretTokens(), "PowerShell", `Get-Content .\secrets\db.yaml`, "secret-token"},
		{shell.SecretTokens(), "PowerShell", `Get-Content C:\home\me\.ssh\id_rsa`, "secret-token"},
		{shell.SecretTokens(), "PowerShell", `Get-Content .\deploy\kubeconfig`, "secret-token"},
		{shell.SecretTokens(), "PowerShell", `Get-Content .\internal\secrets\vault.go`, ""},
		{shell.SecretTokens(), "PowerShell", `Get-Content .\docs\notes.md`, ""},
		{shell.SecretTokens(), "Bash", `cat C:\work\secrets\db.yaml`, ""},
	} {
		got := judgeTool(t, tc.check, tc.tool, tc.command)
		if tc.want == "" && len(got) != 0 || tc.want != "" && (len(got) != 1 || got[0] != tc.want) {
			t.Errorf("%s %q: codes %v, want %q", tc.tool, tc.command, got, tc.want)
		}
	}
}

// TestNoVerifyReadsTheScriptOfInvokeExpression reads the string
// Invoke-Expression runs, by its name or its alias iex in any case and
// after its -Command parameter or an abbreviation of it, as PowerShell
// reads it again; an echo of the same words runs nothing.
func TestNoVerifyReadsTheScriptOfInvokeExpression(t *testing.T) {
	t.Parallel()
	expectCodes(t, shell.NoVerify(), []toolCase{
		{"PowerShell", "Invoke-Expression 'git commit --no-verify -m x'", "no-verify"},
		{"PowerShell", `iex "git commit --no-verify -m x"`, "no-verify"},
		{"PowerShell", "Invoke-Expression -Command 'git commit --no-verify -m x'", "no-verify"},
		{"PowerShell", "IEX -c 'git commit --no-verify -m x'", "no-verify"},
		{"Bash", `pwsh -Command "iex 'git commit --no-verify -m x'"`, "no-verify"},
		{"PowerShell", "Invoke-Expression 'git commit -m x'", ""},
		{"PowerShell", "echo Invoke-Expression git commit --no-verify", ""},
		{"PowerShell", "iex", ""},
	})
}

// TestNoVerifySeesGitBehindReservedWords reads a command behind the
// reserved words a shell puts before it, if, then, elif, else, while,
// until, do, an exclamation mark and an opening brace, and the try and
// finally of PowerShell, the way the shell runs it; the same words as
// arguments of another program and a plain commit behind them pass.
func TestNoVerifySeesGitBehindReservedWords(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		tool, command string
		want          string
	}{
		{"Bash", "if true; then git commit --no-verify -m x; fi", "no-verify"},
		{"Bash", "if git commit --no-verify -m x; then :; fi", "no-verify"},
		{"Bash", "if a; then :; elif b; then :; else git commit -n -m x; fi", "no-verify"},
		{"Bash", "while true; do git commit --no-verify -m x; break; done", "no-verify"},
		{"Bash", "until false; do git commit --no-verify -m x; done", "no-verify"},
		{"Bash", "{ git commit --no-verify -m x; }", "no-verify"},
		{"Bash", "! git commit --no-verify -m x", "no-verify"},
		{"Bash", "if true; then FOO=1 git commit --no-verify -m x; fi", "no-verify"},
		{"PowerShell", "if ($true) { git commit --no-verify -m x }", "no-verify"},
		{"PowerShell", "foreach ($i in 1) { git commit --no-verify -m x }", "no-verify"},
		{"PowerShell", "try { git commit --no-verify -m x } finally { }", "no-verify"},
		{"Bash", "if true; then git commit -m x; fi", ""},
		{"Bash", "echo then git commit --no-verify", ""},
		{"Bash", "{ git status; }", ""},
		{"Bash", "then", ""},
	} {
		got := judgeTool(t, shell.NoVerify(), tc.tool, tc.command)
		if tc.want == "" && len(got) != 0 || tc.want != "" && (len(got) != 1 || got[0] != tc.want) {
			t.Errorf("%s %q: codes %v, want %q", tc.tool, tc.command, got, tc.want)
		}
	}
}
