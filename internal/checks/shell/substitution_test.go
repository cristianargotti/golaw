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

// TestSubstitutionsAreReadInTheirContext reads a substitution where the
// shell expands it: in the body of a here-document whose end word is
// unquoted, where quotes are text, every $( ) and backquote runs, an
// escaped one excepted, while the body of a document whose end word is
// quoted is text, the substitutions it spells included; and the end of a
// substitution is found as the shell finds it, so a parenthesis or a quote
// in the body of a here-document inside it, or the end of a case pattern,
// closes nothing.
func TestSubstitutionsAreReadInTheirContext(t *testing.T) {
	t.Parallel()
	expectCodes(t, shell.NoVerify(), []toolCase{
		{"Bash", "cat <<EOF\n$(git commit --no-verify -m x)\nEOF", "no-verify"},
		{"Bash", "cat <<EOF\n`git commit --no-verify -m x`\nEOF", "no-verify"},
		{"Bash", "cat <<EOF\nsay \"hi\n$(git commit --no-verify -m x)\nEOF", "no-verify"},
		{"Bash", "cat <<EOF\n$(echo a\ngit commit --no-verify -m x)\nEOF", "no-verify"},
		{"Bash", "git commit -m \"$(cat <<EOF\nmsg $(git commit --no-verify -m x)\nEOF\n)\"", "no-verify"},
		{"Bash", "git commit -m \"$(cat <<EOF\nfix: x\n\n1) first\n$(git commit --no-verify -m y)\nEOF\n)\"", "no-verify"},
		{"Bash", "echo \"$(cat <<'EOF'\n)\nEOF\ngit commit --no-verify -m x)\"", "no-verify"},
		{"Bash", "cat > notes.md <<'EOF'\nit's\nEOF\necho \"$(git commit --no-verify -m x)\"", "no-verify"},
		{"Bash", "echo \"$(case $x in a) git commit --no-verify -m x;; esac)\"", "no-verify"},
		{"Bash", "echo \"$(case $x in (a) echo;; b) git commit --no-verify -m x;; esac)\"", "no-verify"},
		{"Bash", "echo `echo \\`date\\`; git commit --no-verify -m x`", "no-verify"},
		{"Bash", "echo `git commit --no-verify -m x", "no-verify"},
		{"Bash", "echo \"`echo \\`date\\`; git commit --no-verify -m x`\"", "no-verify"},
		{"Bash", "git commit -m \"the `echo \"a b\"` form\" --no-verify", "no-verify"},
		{"Bash", "cat <<EOF\n\\$(git commit --no-verify -m x)\nEOF", ""},
		{"Bash", "cat > notes.md <<'EOF'\nthe \"$(git commit --no-verify -m x)\" form\nEOF", ""},
		{"Bash", "git commit -m \"$(cat <<'EOF'\ndocs: say why `git commit --no-verify` is denied\nEOF\n)\"", ""},
		{"Bash", "cat <<\\EOF\n$(git commit --no-verify -m x)\nEOF", ""},
		{"Bash", "cat <<\"EOF\"\n`git commit --no-verify -m x`\nEOF", ""},
	})
	expectCodes(t, shell.SecretTokens(), []toolCase{
		{"Bash", "cat <<EOF\n$(cat .env)\nEOF", "secret-token"},
		{"Bash", "cat > out.txt <<EOF\nK=$(cat .env)\nEOF", "secret-token"},
		{"Bash", "git commit -m \"$(cat <<'EOF'\nfix: read `.env` names\nEOF\n)\"", ""},
		{"Bash", "git commit -F - <<'EOF'\nfix: read `.env` names\nEOF", ""},
		{"Bash", "cat > notes.md <<'EOF'\nCopy `.env.example` to `.env`\nEOF", ""},
		{"Bash", "git commit -m \"$(cat <<'EOF'\nthe \"$(cat .env)\" form\nEOF\n)\"", ""},
		{"Bash", "git commit -m \"$(cat <<'EOF'\nfix: it's done\nEOF\n)\" && echo \"the .env file\"", ""},
	})
}
