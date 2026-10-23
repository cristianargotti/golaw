package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/agents"
)

// turnShellEvent is a shell pre event of the second agent of the registry,
// whose envelope carries a turn id beside the identities of the first.
func turnShellEvent(t *testing.T, cwd, command string) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"session_id": "session", "turn_id": "turn", "agent_id": "agent", "tool_use_id": "shell", "cwd": cwd,
		"hook_event_name": "PreToolUse", "tool_name": "Bash", "tool_input": map[string]any{"command": command},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestHeredocPreEventsAskGitTheShellCount counts the git commands of the
// shell pre events of the second agent of the registry through the real
// binding, the agent whose adapter judges a here-document as a write. A
// plain command and a patch here-document, which its host intercepts and
// follows with no post event, so no baseline is kept for it, ask the five
// questions of the first agent's shell pre event. A cat here-document asks
// one more, its stated limit (docs/agent-contracts.md): the worktree
// listing its write judgment reads beside the status of the baseline its
// post event reads. The test is serial for the reason
// TestHookAsksGitEachFactOnce gives.
func TestHeredocPreEventsAskGitTheShellCount(t *testing.T) {
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	agent := agents.All()[1]
	e := enrolledFor(t, agent)
	const (
		listing = "ls-files -z -t"
		status  = " status "
	)
	for _, tc := range []struct {
		name, command    string
		count            int
		listed, baseline bool
	}{
		{"plain", "go vet ./...", 5, false, true},
		{"patch", "apply_patch <<'EOF'\n*** Begin Patch\n*** Add File: notes.md\n+one-two\n*** End Patch\nEOF\n", 5, true, false},
		{"cat", "cat > notes.md <<'EOF'\none-two\nEOF\n", 6, true, true},
	} {
		if err := os.WriteFile(trace, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		reply := hook.Run(t.Context(), agent.ID, strings.NewReader(turnShellEvent(t, e.root, tc.command)), agent.Adapter, e.deps)
		if reply.ExitCode != 0 || len(reply.Stdout) != 0 || reply.Stderr != "" {
			t.Fatalf("%s: exit %d, stdout %q, stderr %q", tc.name, reply.ExitCode, reply.Stdout, reply.Stderr)
		}
		commands := gitCommandsOf(t, trace)
		asked := func(name string) bool {
			return slices.ContainsFunc(commands, func(c string) bool { return strings.Contains(c, name) })
		}
		sorted := slices.Sorted(slices.Values(commands))
		if len(commands) != tc.count || len(slices.Compact(sorted)) != len(commands) || asked(listing) != tc.listed || asked(status) != tc.baseline {
			t.Fatalf("%s ran %d git commands, want %d:\n%s", tc.name, len(commands), tc.count, strings.Join(commands, "\n"))
		}
	}
}
