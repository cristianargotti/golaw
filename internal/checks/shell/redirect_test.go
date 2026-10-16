package shell_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/shell"
	"github.com/cristianargotti/golaw/internal/model"
)

// toolCase is a command sent through a shell tool and the finding code it
// must yield, empty when it passes.
type toolCase struct {
	tool, command string
	want          string
}

// expectCodes runs one check over every case.
func expectCodes(t *testing.T, c model.Check, cases []toolCase) {
	t.Helper()
	for _, tc := range cases {
		got := judgeTool(t, c, tc.tool, tc.command)
		if tc.want == "" && len(got) != 0 || tc.want != "" && (len(got) != 1 || got[0] != tc.want) {
			t.Errorf("%s %q: codes %v, want %q", tc.tool, tc.command, got, tc.want)
		}
	}
}

// TestRedirectionsAreReadApartFromTheArguments reads a redirection the way
// the shell does before the program runs: an operator outside quotes, its
// stream (digits, a {name} in a POSIX shell, the star of PowerShell), its
// ampersand or pipe (2>&1, &>, >|) and its file leave the words the
// program receives wherever they stand, so the git rules see the arguments
// git gets; a quoted or escaped operator is an argument, and a word an
// operator splits keeps its part before it.
func TestRedirectionsAreReadApartFromTheArguments(t *testing.T) {
	t.Parallel()
	expectCodes(t, shell.NoVerify(), []toolCase{
		{"Bash", "2>/dev/null git commit --no-verify -m x", "no-verify"},
		{"Bash", "git commit -m x 2>&1 --no-verify", "no-verify"},
		{"Bash", "git commit -m x &>/dev/null --no-verify", "no-verify"},
		{"Bash", "git commit -m x >| out.txt --no-verify", "no-verify"},
		{"Bash", "git commit -m x >>out.txt 2>&- --no-verify", "no-verify"},
		{"Bash", "{fd}>out.txt git commit --no-verify -m x", "no-verify"},
		{"Bash", "bash -c > out.txt 'git commit --no-verify -m x'", "no-verify"},
		{"Bash", "git commit -m x>out.txt --no-verify", "no-verify"},
		{"Bash", "git commit -m '>' --no-verify", "no-verify"},
		{"Bash", `git commit -m \> --no-verify`, "no-verify"},
		{"Bash", "git commit -m '2'>out.txt --no-verify", "no-verify"},
		{"Bash", "git commit -m 2&>/dev/null --no-verify", "no-verify"},
		{"Bash", "git commit -m *>/dev/null --no-verify", "no-verify"},
		{"PowerShell", "git commit -m x 2>&1 --no-verify", "no-verify"},
		{"PowerShell", "git commit -m x *>$null --no-verify", "no-verify"},
		{"Bash", `cmd /c 'git commit -m x 2>&1 --no-verify'`, "no-verify"},
		{"Bash", `cmd /c '>nul git commit --no-verify -m x'`, "no-verify"},
		{"Bash", `cmd /c 'git commit -m x>nul --no-verify'`, "no-verify"},
		{"Bash", `cmd /c 'git commit -m a ^> --no-verify'`, "no-verify"},
		{"Bash", `cmd /c 'git commit -m "a > b" --no-verify'`, "no-verify"},
		{"Bash", `cmd /c 'if exist marker >nul git commit --no-verify -m x'`, "no-verify"},
		{"Bash", `cmd /c 'echo x>nul&git commit --no-verify -m x'`, "no-verify"},
		{"Bash", `cmd /c '(echo x>nul) & git commit --no-verify -m x'`, "no-verify"},
		{"Bash", `cmd /c "for /f %i in ('git commit --no-verify -m x 2>nul') do echo %i"`, "no-verify"},
		{"Bash", `cmd /c "for /f %i in ('git status 2>nul') do git commit --no-verify -m x"`, "no-verify"},
	})
}

// TestRedirectionsLeaveTheirFilesToTheShell passes a commit whose bypass
// flag the shell takes as the file of a redirection or leaves to -m as its
// message, a here-string, a background job before an echo, and an
// operator that waits for its file at the end of the line.
func TestRedirectionsLeaveTheirFilesToTheShell(t *testing.T) {
	t.Parallel()
	expectCodes(t, shell.NoVerify(), []toolCase{
		{"Bash", "git commit -m x > out.txt", ""},
		{"Bash", "git commit -m > out.txt --no-verify", ""},
		{"Bash", "git commit -m x <<< --no-verify", ""},
		{"Bash", "git commit -m x & >out.txt echo --no-verify", ""},
		{"Bash", "git commit -m x > --no-verify", ""},
		{"Bash", "cat <(git status) >", ""},
		{"Bash", `cmd /c 'git commit -m >nul --no-verify'`, ""},
		{"Bash", `cmd /c 'git commit -m x 2>"--no-verify"'`, ""},
	})
}

// TestSecretTokensJudgeTheFileOfARedirection judges the file of every
// redirection by name, joined to its operator or not, and assigns the
// package roles of the go tool on the words it receives: a redirection
// between a flag and its value leaves the value to the flag, and a quoted
// operator is a word the program receives.
func TestSecretTokensJudgeTheFileOfARedirection(t *testing.T) {
	t.Parallel()
	expectCodes(t, shell.SecretTokens(), []toolCase{
		{"Bash", "go run -tags < ./deploy/secrets/... example ./readstdin.go", "secret-token"},
		{"Bash", "go vet -tags < ./deploy/secrets/... example ./x/...", "secret-token"},
		{"Bash", "<./deploy/secrets/... go vet ./x/...", "secret-token"},
		{"Bash", "go test -run X '>' -args ./deploy/secrets/...", "secret-token"},
		{"Bash", "echo x 2>.env", "secret-token"},
		{"Bash", "go build ./... >| ./deploy/secrets/...", "secret-token"},
		{"PowerShell", "go vet ./... 2> .\\deploy\\secrets\\...", "secret-token"},
		{"Bash", `cmd /c 'echo x>.env'`, "secret-token"},
		{"Bash", `cmd /c 'type nul 2> ".env"'`, "secret-token"},
		{"Bash", "2>/dev/null go vet ./internal/secrets/...", ""},
		{"Bash", "go vet -tags '<' ./internal/secrets/...", ""},
		{"Bash", "go test ./internal/secrets/... 2>&1 > out.txt", ""},
		{"Bash", `cmd /c 'go vet ./internal/secrets/... 2>nul'`, ""},
	})
}
