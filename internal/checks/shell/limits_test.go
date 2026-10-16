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
// first are read as one command with the words before them; and a
// substitution inside double quotes ends where bash 4, zsh and dash end
// it, so a line bash 3.2 would end at a parenthesis of a here-document body
// inside it, and then read on as commands, is read as the newer shells
// read it.
func TestNoVerifyReadsTheLineAsWritten(t *testing.T) {
	t.Parallel()
	expectCodes(t, shell.NoVerify(), []toolCase{
		{"Bash", "echo \"$(cat <<'EOF'\n) \" ; git commit --no-verify -m y ; echo \" (\nEOF\n)\"", ""},
		{"Bash", "echo \"$(cat <<'EOF'\n)\nEOF\ngit commit --no-verify -m x)\"", "no-verify"},
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

// TestHereDocumentBodiesOtherProgramsReadAreData pins the limit
// docs/security.md states for here-documents: a body is read as a script
// only when its line names a shell or sources the standard input, so a
// body eval runs through the output of cat, one xargs or a read loop turns
// into arguments and the script of another interpreter pass, for the hook
// bypass flag and for the secret names, beside their counterparts the hook
// reads.
func TestHereDocumentBodiesOtherProgramsReadAreData(t *testing.T) {
	t.Parallel()
	expectCodes(t, shell.NoVerify(), []toolCase{
		{"Bash", "eval \"$(cat)\" <<'EOF'\ngit commit --no-verify -m x\nEOF", ""},
		{"Bash", "xargs git commit -m x <<'EOF'\n--no-verify\nEOF", ""},
		{"Bash", "python3 - <<'EOF'\nimport os; os.system('git commit --no-verify -m x')\nEOF", ""},
		{"Bash", "bash <<'EOF'\ngit commit --no-verify -m x\nEOF", "no-verify"},
		{"Bash", "source /dev/stdin <<'EOF'\ngit commit --no-verify -m x\nEOF", "no-verify"},
	})
	expectCodes(t, shell.SecretTokens(), []toolCase{
		{"Bash", "xargs cat <<'EOF'\n.env\nEOF", ""},
		{"Bash", "while read f; do cat \"$f\"; done <<'EOF'\n.env\nEOF", ""},
		{"Bash", "python3 - <<'EOF'\nprint(open('.env').read())\nEOF", ""},
		{"Bash", "bash <<'EOF'\ncat .env\nEOF", "secret-token"},
	})
}

// TestExpansionsTheScannerEndsAreAStatedLimit pins the limit
// docs/security.md states for a << inside a $[ ] or ${ }: the reader
// follows one only within the word being built, so one that holds a blank,
// a parenthesis or another separator of commands ends with the word there,
// and a << after that point opens a here-document whose body hides the
// next line, beside the counterpart the reader follows.
func TestExpansionsTheScannerEndsAreAStatedLimit(t *testing.T) {
	t.Parallel()
	expectCodes(t, shell.NoVerify(), []toolCase{
		{"Bash", "echo ${x:-a b<<c}\ngit commit --no-verify -m x", ""},
		{"Bash", "echo ${x:-$(true)<<b}\ngit commit --no-verify -m x", ""},
		{"Bash", "echo ${x:-a;b<<c}\ngit commit --no-verify -m x", ""},
		{"Bash", "echo $[1 <<2]\ngit commit --no-verify -m x", ""},
		{"Bash", "echo ${x:-a<<b}\ngit commit --no-verify -m x", "no-verify"},
	})
}

// TestCommentAfterASubstitutionIsAStatedLimit pins the first of the four
// shapes docs/security.md states a well-meaning agent does not write: a
// hash right after the parenthesis that closes a substitution, an
// arithmetic expansion or a process substitution inside a word begins a
// comment for this reading, while the shell keeps it in the word, so the
// commands after it on its line pass and a quote after it opens nothing,
// beside the counterparts the reader follows: a substitution inside double
// quotes keeps the word open, and a blank before the hash makes it a
// comment for the shell too.
func TestCommentAfterASubstitutionIsAStatedLimit(t *testing.T) {
	t.Parallel()
	expectCodes(t, shell.NoVerify(), []toolCase{
		{"Bash", "x=$(true)#; git commit --no-verify -m x", ""},
		{"Bash", "echo $((1))#; git commit --no-verify -m x", ""},
		{"Bash", "cat <(true)#; git commit --no-verify -m x", ""},
		{"Bash", "x=$(true)#'\ngit commit --no-verify -m x\n'", "no-verify"},
		{"Bash", "echo \"$(true)\"#; git commit --no-verify -m x", "no-verify"},
		{"Bash", "x=$(true) #; git commit --no-verify -m x", ""},
	})
	expectCodes(t, shell.SecretTokens(), []toolCase{
		{"Bash", "x=$(true)#; cat .env", ""},
		{"Bash", "x=$(true); cat .env", "secret-token"},
	})
}

// TestWordsAfterTheScriptOfAShellAreAStatedLimit pins the second stated
// shape: the words after the script of sh -c or bash -c, which the shell
// hands the script as its positional parameters, are read as words of the
// script, so a comment that ends the script hides them, while without the
// comment they are judged.
func TestWordsAfterTheScriptOfAShellAreAStatedLimit(t *testing.T) {
	t.Parallel()
	expectCodes(t, shell.NoVerify(), []toolCase{
		{"Bash", "bash -c 'git commit \"$@\" #' sh --no-verify -m x", ""},
		{"Bash", "bash -c 'git commit \"$@\"' sh --no-verify -m x", "no-verify"},
	})
	expectCodes(t, shell.SecretTokens(), []toolCase{
		{"Bash", "bash -c 'cat \"$1\" # read' sh .env", ""},
		{"Bash", "sh -c 'cat \"$1\" #' sh .env", ""},
		{"Bash", "bash -c 'cat \"$1\"' sh .env", "secret-token"},
	})
}

// TestEscapesOfEnvSAreAStatedLimit pins the third stated shape: the string
// of env -S is split as a POSIX shell splits words, so an escape env reads
// its own way is read as the character it escapes: \_, a blank to env,
// joins two words into one name, here a source file or a name the list
// does not hold, and \c, the end of the string to env, is read as a c, so
// the name after it is still judged.
func TestEscapesOfEnvSAreAStatedLimit(t *testing.T) {
	t.Parallel()
	expectCodes(t, shell.SecretTokens(), []toolCase{
		{"Bash", `env -S 'cat ./secrets/foo\_bar.go harmless.txt'`, ""},
		{"Bash", `env -S 'cat .env\_x.go'`, ""},
		{"Bash", `env -S 'cat \c .env'`, "secret-token"},
		{"Bash", `env -S 'cat .env x.go'`, "secret-token"},
	})
}

// TestBash32LineFedToBashIsAStatedLimit pins the fourth stated shape: a
// line crafted for bash 3.2, which ends a substitution inside double quotes
// at the first parenthesis left open, is read as bash 4 and later read it,
// typed at the tool's shell (TestNoVerifyReadsTheLineAsWritten) or, as
// here, fed to bash through a here-document, beside the counterpart whose
// git call the newer shells run too.
func TestBash32LineFedToBashIsAStatedLimit(t *testing.T) {
	t.Parallel()
	expectCodes(t, shell.NoVerify(), []toolCase{
		{"Bash", "bash <<'X'\necho \"$(cat <<'EOF'\n) \" ; git commit --no-verify -m y ; echo \" (\nEOF\n)\"\nX", ""},
		{"Bash", "bash <<'X'\necho \"$(cat <<'EOF'\n)\nEOF\ngit commit --no-verify -m x)\"\nX", "no-verify"},
	})
}
