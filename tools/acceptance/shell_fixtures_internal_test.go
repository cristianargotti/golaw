package main

import "testing"

func TestShellFixturePairs(t *testing.T) {
	t.Parallel()
	seen := make(map[string]string)
	for _, item := range shellFixtures() {
		seen[item.name] = item.want
		if item.event != eventPre ||
			item.tool != toolBash && item.tool != toolPowerShell ||
			item.input[keyCommand] == "" {
			t.Fatalf("%+v", item)
		}
	}
	denied := []string{
		"bypass", "short-bypass", "commit-dash", "body-101",
		"secret-token", "pwsh", "powershell", "cmd", "bash", "sh",
	}
	allowed := []string{
		"commit-control", "push-default", "fetch", "body-100", "source-token",
		"pwsh-control", "powershell-control", "cmd-control", "bash-control", "sh-control",
	}
	for _, tool := range []string{toolBash, toolPowerShell} {
		expectShell(t, seen, denied, tool, wantDeny)
		expectShell(t, seen, allowed, tool, wantAllow)
	}
}

// expectShell asserts the expectation of every named shell fixture of a tool.
func expectShell(t *testing.T, seen map[string]string, names []string, tool, want string) {
	t.Helper()
	for _, name := range names {
		if seen[name+"-"+tool] != want {
			t.Fatal(name, tool)
		}
	}
}
