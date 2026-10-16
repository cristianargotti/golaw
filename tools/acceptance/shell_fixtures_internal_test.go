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
		"secret-token", "secret-directory", "package-redirect", "package-terminator-test", "redirect-flag-value", "redirect-bypass", "pwsh", "powershell", "cmd", "cmd-escaped-quote", "cmd-control-block", "cmd-for-set", "cmd-for-command", "cmd-call", "cmd-start", "cmd-start-run", "reserved-word-bypass", "eval-bypass", "xargs-bypass", "git-dir-bypass", "wrapper-option-bypass", "split-string-value", "file-before-script", "invoke-expression-bypass", "substitution-bypass", "comment-quote-bypass", "bash", "sh",
	}
	allowed := []string{
		"commit-control", "push-default", "fetch", "body-100", "source-token", "package-pattern", "package-terminator", "redirect-control",
		"pwsh-control", "powershell-control", "cmd-control", "cmd-escaped-quote-control", "cmd-control-block-control", "cmd-for-set-control", "cmd-for-command-control", "cmd-call-control", "cmd-start-run-control",
		"reserved-word-control", "comment-control", "eval-control", "split-string-control", "file-before-script-control", "bash-control", "sh-control",
	}
	for _, tool := range []string{toolBash, toolPowerShell} {
		expectShell(t, seen, denied, tool, wantDeny)
		expectShell(t, seen, allowed, tool, wantAllow)
	}
	expectShell(t, seen, []string{"native-bypass", "native-call-bypass", "native-secret", "native-relative-secret"}, toolPowerShell, wantDeny)
	expectShell(t, seen, []string{"native-commit-control", "native-source"}, toolPowerShell, wantAllow)
	if _, found := seen["native-bypass-"+toolBash]; found {
		t.Fatal("a native PowerShell row runs through Bash")
	}
	expectShell(t, seen, []string{"heredoc-script", "heredoc-substitution", "arithmetic-shift-bypass", "heredoc-source"}, toolBash, wantDeny)
	expectShell(t, seen, []string{"heredoc-data", "heredoc-message", "heredoc-quoted-substitution", "arithmetic-shift-control", "heredoc-source-control"}, toolBash, wantAllow)
	if _, found := seen["heredoc-data-"+toolPowerShell]; found {
		t.Fatal("a POSIX row runs through PowerShell")
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
