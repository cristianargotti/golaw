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

// TestHereDocumentsSourcedFromTheStandardInputAreScripts reads the body of
// a here-document its line sources from the standard input, source or dot
// of /dev/stdin, /dev/fd/0 or /proc/self/fd/0, behind the wrappers too, as
// a script of the shell itself, while a sourced file leaves the body the
// input it is.
func TestHereDocumentsSourcedFromTheStandardInputAreScripts(t *testing.T) {
	t.Parallel()
	expectCodes(t, shell.NoVerify(), []toolCase{
		{"Bash", "source /dev/stdin <<'EOF'\ngit commit --no-verify -m x\nEOF", "no-verify"},
		{"Bash", ". /dev/stdin <<'EOF'\ngit commit --no-verify -m x\nEOF", "no-verify"},
		{"Bash", "source /proc/self/fd/0 <<'EOF'\ngit commit --no-verify -m x\nEOF", "no-verify"},
		{"Bash", "builtin source /dev/fd/0 <<EOF\ngit commit --no-verify -m x\nEOF", "no-verify"},
		{"Bash", "cat <<'EOF' | . /dev/stdin\ngit commit --no-verify -m x\nEOF", "no-verify"},
		{"Bash", ". ./env.sh <<'EOF'\ngit commit --no-verify -m x\nEOF", ""},
		{"Bash", "echo source /dev/stdin <<'EOF'\ngit commit --no-verify -m x\nEOF", ""},
		{"Bash", "source <<'EOF'\ngit commit --no-verify -m x\nEOF", ""},
	})
	expectCodes(t, shell.SecretTokens(), []toolCase{
		{"Bash", "source /dev/fd/0 <<'EOF'\ncat .env\nEOF", "secret-token"},
	})
}

// TestHereDocumentsOpenWhereTheShellOpensThem keeps expansion text from
// hiding the next command in a false body, while a real body that never
// ends remains data unless a shell reads it or expands its substitutions.
func TestHereDocumentsOpenWhereTheShellOpensThem(t *testing.T) {
	t.Parallel()
	expectCodes(t, shell.NoVerify(), []toolCase{
		{"Bash", "echo $((1<<2))\ngit commit --no-verify -m x", "no-verify"},
		{"Bash", "(( y = 1 << 3 ))\ngit commit --no-verify -m x", "no-verify"},
		{"Bash", "echo $(( (1<<2) > 3 ))\ngit commit --no-verify -m x", "no-verify"},
		{"Bash", "echo \"$((1<<2))\"\ngit commit --no-verify -m x", "no-verify"},
		{"Bash", "echo \"$((1<<2\n))\"\ngit commit --no-verify -m x", "no-verify"},
		{"Bash", "echo $[1<<2]\ngit commit --no-verify -m x", "no-verify"},
		{"Bash", "echo ${x:-a<<b}\ngit commit --no-verify -m x", "no-verify"},
		{"Bash", "echo ${x:-a<<b}\ngit commit --no-verify -m x\nb}", "no-verify"},
		{"Bash", "echo $[1<<2]\ngit commit --no-verify -m x\n2]", "no-verify"},
		{"Bash", "echo ${x:-${y}<<b}\ngit commit --no-verify -m x", "no-verify"},
		{"Bash", "echo $[a[1]<<2]\ngit commit --no-verify -m x\n2]", "no-verify"},
		{"Bash", "echo ${x:-]a<<b}\ngit commit --no-verify -m x\nb}", "no-verify"},
		{"Bash", "cat <<'EOF'\ngit commit --no-verify -m x", ""},
		{"Bash", "cat <<A <<B\nx\nA\ngit commit --no-verify -m y", ""},
		{"Bash", "echo \\${x<<EOF\ngit commit --no-verify -m x\nEOF", ""},
		{"Bash", "echo '$'{x<<'EOF'\ngit commit --no-verify -m x\nEOF", ""},
		{"Bash", "cat <<EOF\n$(git commit --no-verify -m x)", "no-verify"},
		{"Bash", "cat <<'EOF'\n$(git commit --no-verify -m x)", ""},
		{"Bash", "echo $(( $(git commit --no-verify -m x | wc -l) + 1 ))", "no-verify"},
		{"Bash", "cat > run.sh <<'EOF'\ngit commit --no-verify -m x\nEOF", ""},
		{"Bash", "n=$((1+2))\ngit commit -m x", ""},
	})
	expectCodes(t, shell.SecretTokens(), []toolCase{
		{"Bash", "x=$(( 1 << 2 ))\ncat .env", "secret-token"},
		{"Bash", "cat <<'EOF'\ncat .env", ""},
		{"Bash", "bash <<'EOF'\ncat .env", "secret-token"},
		{"Bash", "echo ${x:-a<<b}>.env", "secret-token"},
		{"Bash", "echo $(( 2 > 1 )) > out.txt", ""},
		{"Bash", "cat <<A <<B\nthe .env file\nA\nnotes", ""},
	})
}
