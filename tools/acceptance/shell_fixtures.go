package main

import "strings"

const overBodyWidth = 101

type shellFixture struct {
	name string
	text string
	want string
}

func shellCommands() []shellFixture {
	return []shellFixture{
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
		{"secret-directory", "grep -R . deploy/secrets/db/", wantDeny},
		{"pwsh", `pwsh -Command "git commit --no-verify -m 'fix: safe'"`, wantDeny},
		{"powershell", `powershell -NoProfile -Command "git commit --no-verify -m 'fix: safe'"`, wantDeny},
		{"cmd", `cmd /c "git commit --no-verify -m 'fix: safe'"`, wantDeny},
		{"cmd-escaped-quote", `cmd /c 'git commit -m "fix: \"" --no-verify'`, wantDeny},
		{"cmd-escaped-quote-control", `cmd /c 'git commit -m "fix: \" --no-verify"'`, wantAllow},
		{"bash", `bash -lc "git commit --no-verify -m 'fix: safe'"`, wantDeny},
		{"sh", `sh -c "git commit --no-verify -m 'fix: safe'"`, wantDeny},
		{"pwsh-control", `pwsh -Command "git status"`, wantAllow},
		{"powershell-control", `powershell -NoProfile -Command "git status"`, wantAllow},
		{"cmd-control", `cmd /c "git status"`, wantAllow},
		{"bash-control", `bash -lc "git status"`, wantAllow},
		{"sh-control", `sh -c "git status"`, wantAllow},
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

func shellFixtures() []fixture {
	commands, native := shellCommands(), nativeCommands()
	out := make([]fixture, 0, 2*len(commands)+len(native))
	for _, tool := range []string{toolBash, toolPowerShell} {
		for _, item := range commands {
			out = append(out, shellFixtureOf(item, tool))
		}
	}
	for _, item := range native {
		out = append(out, shellFixtureOf(item, toolPowerShell))
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
