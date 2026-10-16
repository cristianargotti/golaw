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
		{"pwsh", `pwsh -Command "git commit --no-verify -m 'fix: safe'"`, wantDeny},
		{"powershell", `powershell -NoProfile -Command "git commit --no-verify -m 'fix: safe'"`, wantDeny},
		{"cmd", `cmd /c "git commit --no-verify -m 'fix: safe'"`, wantDeny},
		{"bash", `bash -lc "git commit --no-verify -m 'fix: safe'"`, wantDeny},
		{"sh", `sh -c "git commit --no-verify -m 'fix: safe'"`, wantDeny},
		{"pwsh-control", `pwsh -Command "git status"`, wantAllow},
		{"powershell-control", `powershell -NoProfile -Command "git status"`, wantAllow},
		{"cmd-control", `cmd /c "git status"`, wantAllow},
		{"bash-control", `bash -lc "git status"`, wantAllow},
		{"sh-control", `sh -c "git status"`, wantAllow},
	}
}

func shellFixtures() []fixture {
	commands := shellCommands()
	out := make([]fixture, 0, 2*len(commands))
	for _, tool := range []string{toolBash, toolPowerShell} {
		for _, item := range commands {
			out = append(out, fixture{
				name: item.name + "-" + tool, category: "shell",
				event: eventPre, tool: tool, want: item.want,
				input: map[string]any{keyCommand: item.text},
			})
		}
	}
	return out
}
