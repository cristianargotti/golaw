package main

import "strings"

const overBodyWidth = 101

// startRun is the number of start words of the repeated start rows, enough
// for a reading that doubles at each start to spend the hook budget.
const startRun = 30

type shellFixture struct {
	name string
	text string
	want string
}

func shellCommands() []shellFixture {
	return append([]shellFixture{
		{"bypass", "git commit --no-verify -m 'fix: safe'", wantDeny},
		{"short-bypass", "git commit -n -m 'fix: safe'", wantDeny},
		{"commit-control", "git commit -m 'fix: safe'", wantAllow},
		{"commit-dash", "git commit -m 'fix: one\u2014two'", wantDeny},
		{"push-default", "git push", wantAllow},
		{"fetch", "git fetch", wantAllow},
		{"body-100", "git commit -m 'fix: safe' -m '" + strings.Repeat("x", 100) + "'", wantAllow},
		{"body-101", "git commit -m 'fix: safe' -m '" + strings.Repeat("x", overBodyWidth) + "'", wantDeny},
		{"secret-token", "cat .env", wantDeny},
		{"source-token", "cat credentials.go", wantAllow},
		{"package-pattern", "go test ./internal/secrets/...", wantAllow},
		{"package-terminator", "go build -- ./internal/secrets/...", wantAllow},
		{"package-terminator-test", "go test -- ./internal/secrets/...", wantDeny},
		{"package-redirect", "go run ./x.go < ./deploy/secrets/...", wantDeny},
		{"redirect-flag-value", "go run -tags < ./deploy/secrets/... example ./readstdin.go", wantDeny},
		{"redirect-bypass", "git commit -m 'fix: safe' 2>&1 --no-verify", wantDeny},
		{"redirect-control", "git commit -m 'fix: safe' > out.txt", wantAllow},
		{"secret-directory", "grep -R . deploy/secrets/db/", wantDeny},
		{"pwsh", `pwsh -Command "git commit --no-verify -m 'fix: safe'"`, wantDeny},
		{"powershell", `powershell -NoProfile -Command "git commit --no-verify -m 'fix: safe'"`, wantDeny},
		{"reserved-word-bypass", "if true; then git commit --no-verify -m 'fix: safe'; fi", wantDeny},
		{"reserved-word-control", "if true; then git commit -m 'fix: safe'; fi", wantAllow},
		{"substitution-bypass", `git commit -m "$(git commit --no-verify -m y)"`, wantDeny},
		{"bash", `bash -lc "git commit --no-verify -m 'fix: safe'"`, wantDeny},
		{"sh", `sh -c "git commit --no-verify -m 'fix: safe'"`, wantDeny},
		{"pwsh-control", `pwsh -Command "git status"`, wantAllow},
		{"powershell-control", `powershell -NoProfile -Command "git status"`, wantAllow},
		{"bash-control", `bash -lc "git status"`, wantAllow},
		{"sh-control", `sh -c "git status"`, wantAllow},
	}, append(wrapperCommands(), cmdScripts()...)...)
}

// wrapperCommands are the commands that reach their program past eval,
// Invoke-Expression, xargs, env and the global options of git, and the
// words a wrapper reads before the script of a shell, each case with its
// control.
func wrapperCommands() []shellFixture {
	return []shellFixture{
		{"eval-bypass", `eval "git commit --no-verify -m 'fix: safe'"`, wantDeny},
		{"xargs-bypass", "echo | xargs git commit --no-verify -m x", wantDeny},
		{"eval-control", `eval "git commit -m 'fix: safe'"`, wantAllow},
		{"git-dir-bypass", "git --git-dir .git commit --no-verify -m 'fix: safe'", wantDeny},
		{"wrapper-option-bypass", "env -i git commit --no-verify -m 'fix: safe'", wantDeny},
		{"split-string-value", "env -S 'go build -overlay ./deploy/secrets/overlay.json ./internal/...'", wantDeny},
		{"split-string-control", "env -S 'go build -tags fast ./...'", wantAllow},
		{"file-before-script", "xargs -a .env sh -c 'cat'", wantDeny},
		{"file-before-script-control", "xargs -a list.txt sh -c 'cat'", wantAllow},
		{"invoke-expression-bypass", `Invoke-Expression "git commit --no-verify -m 'fix: safe'"`, wantDeny},
	}
}

// cmdScripts are the commands that run a script through cmd /c, each case
// of the cmd reading with its control.
func cmdScripts() []shellFixture {
	return []shellFixture{
		{"cmd", `cmd /c "git commit --no-verify -m 'fix: safe'"`, wantDeny},
		{"cmd-escaped-quote", `cmd /c 'git commit -m "fix: \"" --no-verify'`, wantDeny},
		{"cmd-escaped-quote-control", `cmd /c 'git commit -m "fix: \" --no-verify"'`, wantAllow},
		{"cmd-control-block", `cmd /c 'if exist marker (git commit --no-verify -m "fix: x")'`, wantDeny},
		{"cmd-control-block-control", `cmd /c 'if exist marker (git commit -m "fix: x")'`, wantAllow},
		{"cmd-for-set", `cmd /c 'for /f %i in ( .env ) do echo %i'`, wantDeny},
		{"cmd-for-set-control", `cmd /c 'for %i in (a.txt b.txt) do type %i'`, wantAllow},
		{"cmd-for-command", `cmd /c "for /f %i in ('git commit --no-verify -m x') do echo %i"`, wantDeny},
		{"cmd-for-command-control", `cmd /c "for /f %i in ('git log -1') do echo %i"`, wantAllow},
		{"cmd-call", `cmd /c 'call git commit --no-verify -m x'`, wantDeny},
		{"cmd-start", `cmd /c 'start /b git commit --no-verify -m x'`, wantDeny},
		{"cmd-call-control", `cmd /c 'call git commit -m x'`, wantAllow},
		{"cmd-start-run", "cmd /c '" + strings.Repeat("start ", startRun) + "git commit -n -m x'", wantDeny},
		{"cmd-start-run-control", "cmd /c '" + strings.Repeat("start ", startRun) + "git status'", wantAllow},
		{"cmd-control", `cmd /c "git status"`, wantAllow},
	}
}

// nativeCommands are the PowerShell commands that name Windows paths the
// way PowerShell reads them, with the backslash as the separator: a git
// called by its full path, through the call operator too, and secret
// material named by an absolute or a relative native path, each with its
// control.
func nativeCommands() []shellFixture {
	return []shellFixture{
		{"native-bypass", `C:\Git\cmd\git.exe commit --no-verify -m 'fix: safe'`, wantDeny},
		{"native-call-bypass", `& "C:\Program Files\Git\cmd\git.exe" commit -n -m 'fix: safe'`, wantDeny},
		{"native-commit-control", `& "C:\Program Files\Git\cmd\git.exe" commit -m 'fix: safe'`, wantAllow},
		{"native-secret", `Get-Content C:\work\secrets\db.yaml`, wantDeny},
		{"native-relative-secret", `Get-Content .\deploy\kubeconfig`, wantDeny},
		{"native-source", `Get-Content .\internal\secrets\vault.go`, wantAllow},
	}
}

// posixCommands are the Bash commands whose reading PowerShell does not
// share: the body of a here-document is the stdin of its command, data,
// unless a shell its line names reads it as a script.
func posixCommands() []shellFixture {
	return []shellFixture{
		{"heredoc-data", "cat > notes.md <<'EOF'\nread the .env example\nEOF", wantAllow},
		{"heredoc-script", "bash <<'EOF'\ngit commit --no-verify -m x\nEOF", wantDeny},
		{"heredoc-message", "git commit -m \"$(cat <<'EOF'\nfix: read the .env example\nEOF\n)\"", wantAllow},
	}
}

func shellFixtures() []fixture {
	commands, native, posix := shellCommands(), nativeCommands(), posixCommands()
	out := make([]fixture, 0, 2*len(commands)+len(native)+len(posix))
	for _, tool := range []string{toolBash, toolPowerShell} {
		for _, item := range commands {
			out = append(out, shellFixtureOf(item, tool))
		}
	}
	for _, item := range native {
		out = append(out, shellFixtureOf(item, toolPowerShell))
	}
	for _, item := range posix {
		out = append(out, shellFixtureOf(item, toolBash))
	}
	return out
}

func shellFixtureOf(item shellFixture, tool string) fixture {
	return fixture{
		name: item.name + "-" + tool, category: "shell",
		event: eventPre, tool: tool, want: item.want,
		input: map[string]any{keyCommand: item.text},
	}
}
