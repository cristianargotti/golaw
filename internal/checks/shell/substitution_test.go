package shell_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/shell"
)

// TestSubstitutionsInsideAWordAreRead reads the command a substitution
// runs where the scanner keeps it inside a word: $( ) inside double quotes,
// in a POSIX shell and in PowerShell, up to its matching parenthesis past
// quoted ones and nested substitutions, and the backquotes of a POSIX
// shell, outside quotes or inside double ones; a single quoted or escaped
// substitution is text.
func TestSubstitutionsInsideAWordAreRead(t *testing.T) {
	t.Parallel()
	expectCodes(t, shell.NoVerify(), []toolCase{
		{"Bash", `git commit -m "$(git commit --no-verify -m y)"`, "no-verify"},
		{"Bash", "echo `git commit --no-verify -m x`", "no-verify"},
		{"Bash", "echo \"`git commit --no-verify -m x`\"", "no-verify"},
		{"Bash", `echo "$(echo "a)b"; git commit --no-verify -m x)"`, "no-verify"},
		{"Bash", `echo "$(echo $(git commit --no-verify -m x))"`, "no-verify"},
		{"Bash", `echo "$(echo \); git commit --no-verify -m x)"`, "no-verify"},
		{"Bash", `echo "$(git commit --no-verify -m x`, "no-verify"},
		{"PowerShell", `echo "$(git commit --no-verify -m x)"`, "no-verify"},
		{"Bash", `echo '$(git commit --no-verify -m x)'`, ""},
		{"Bash", `echo "\$(git commit --no-verify -m x)"`, ""},
		{"Bash", "echo '`git commit --no-verify -m x`'", ""},
		{"Bash", "git commit -m \"the \\`x\\` flag\"", ""},
		{"Bash", "git commit -m \"the `--no-verify` flag\"", ""},
		{"PowerShell", `echo '$(git commit --no-verify -m x)'`, ""},
		{"PowerShell", "echo \"`$(git commit --no-verify -m x)\"", ""},
	})
	expectCodes(t, shell.SecretTokens(), []toolCase{
		{"Bash", `echo "$(cat .env)"`, "secret-token"},
		{"Bash", "echo `cat .env`", "secret-token"},
		{"Bash", `echo "$(cat notes.md)"`, ""},
		{"Bash", "git commit -m \"$(cat <<'EOF'\nfix: read the .env example\n\nThe id_rsa of the docs.\nEOF\n)\"", ""},
	})
}
