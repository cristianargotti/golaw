package shell_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/shell"
)

// TestHeredocBodiesAreTheirCommandsInput reads the lines after a line that
// opens here-documents (<< and <<-, quoted end words included) as their
// bodies, the stdin of the command: data, never commands, unless a word of
// that line names a shell, which then reads a body as its script; a line
// equal to the end word, its tabs stripped under <<-, ends a body, the
// documents of one line are read in order, and the line after the last
// body is a command again.
func TestHeredocBodiesAreTheirCommandsInput(t *testing.T) {
	t.Parallel()
	expectCodes(t, shell.NoVerify(), []toolCase{
		{"Bash", "bash <<'EOF'\ngit commit --no-verify -m x\nEOF", "no-verify"},
		{"Bash", "cat <<EOF | sh\ngit commit --no-verify -m x\nEOF", "no-verify"},
		{"Bash", "cat <<-EOF | bash\n\tgit commit --no-verify -m x\n\tEOF", "no-verify"},
		{"Bash", "cat <<A <<B | sh\nx\nA\ngit commit --no-verify -m y\nB", "no-verify"},
		{"Bash", "bash <<'EOF'\ngit commit --no-verify -m x", "no-verify"},
		{"Bash", "cat <<'EOF'\ngit status\nEOF\ngit commit --no-verify -m x", "no-verify"},
		{"Bash", "cat > run.sh <<'EOF'\ngit commit --no-verify -m x\nEOF", ""},
		{"Bash", "cat <<'EOF'\ngit commit --no-verify -m x\nEOF\ngit status", ""},
		{"Bash", "cat <<EOF\r\ngit commit --no-verify -m x\r\nEOF\r\n", ""},
		{"Bash", "git commit -m x <<< 'git commit --no-verify'", ""},
	})
	expectCodes(t, shell.SecretTokens(), []toolCase{
		{"Bash", "cat > notes.md <<'EOF'\nread the .env example\nEOF", ""},
		{"Bash", "x=$(cat <<'EOF'\nthe .env file\nEOF\n)", ""},
		{"Bash", "cat <<'EOF' > .env\nK=v\nEOF", "secret-token"},
		{"Bash", "bash <<'EOF'\ncat .env\nEOF", "secret-token"},
		{"Bash", "cat <<EOF | sh\ncat .env\nEOF", "secret-token"},
		{"Bash", "cat <<'EOF'\nnotes\nEOF\ncat .env", "secret-token"},
	})
}
