package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
)

// shellEventAt is a shell event of the agent at a working directory: the
// pre event, or the post event of the same tool use with its response.
func shellEventAt(t *testing.T, name, cwd, command string) string {
	t.Helper()
	event := map[string]any{
		"session_id": "session", "agent_id": "agent", "tool_use_id": "shell", "cwd": cwd,
		"hook_event_name": name, "tool_name": "Bash",
		"tool_input": map[string]any{"command": command, "description": "probe"},
	}
	if name == "PostToolUse" {
		event["tool_response"] = map[string]any{"stdout": "", "stderr": "", "interrupted": false, "isImage": false}
	}
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// gitCommandsOf reads the trace git writes for every command it runs and
// answers one line per git process: the command it ran.
func gitCommandsOf(t *testing.T, trace string) []string {
	t.Helper()
	data, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	var commands []string
	for line := range strings.SplitSeq(string(data), "\n") {
		if _, command, found := strings.Cut(line, "trace: built-in: "); found {
			commands = append(commands, strings.TrimSpace(command))
		}
	}
	return commands
}

// TestHookAsksGitEachFactOnce counts the git commands one event of each
// kind starts through the real binding, from the trace git writes for every
// command it runs: no event asks git the same question twice, a Read lists
// no worktree, a shell event that changes nothing identifies the changes
// through git status instead of listing the worktree, and the counts
// reached are the ceiling. The test is serial because the trace and the
// isolated git configuration live in the environment the git processes
// inherit.
func TestHookAsksGitEachFactOnce(t *testing.T) {
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	e := enrolledHook(t)
	agent := agentUnderTest()
	for _, tc := range []struct {
		name, event, listing string
		ceiling              int
	}{
		{"read", eventAt(t, "Read", e.root, map[string]any{"file_path": filepath.Join(e.root, "go.mod")}), "ls-files -z -t", 4},
		{"write", eventAt(t, "Write", e.root, map[string]any{"file_path": filepath.Join(e.root, "notes.md"), "content": "one-two\n"}), "", 5},
		{"bash pre", shellEventAt(t, "PreToolUse", e.root, "go vet ./..."), "ls-files -z -t", 5},
		{"bash post", shellEventAt(t, "PostToolUse", e.root, "go vet ./..."), "ls-files -z -t", 5},
	} {
		if err := os.WriteFile(trace, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		reply := hook.Run(t.Context(), agent.ID, strings.NewReader(tc.event), agent.Adapter, e.deps)
		if reply.ExitCode != 0 || len(reply.Stdout) != 0 || reply.Stderr != "" {
			t.Fatalf("%s: exit %d, stdout %q, stderr %q", tc.name, reply.ExitCode, reply.Stdout, reply.Stderr)
		}
		commands := gitCommandsOf(t, trace)
		sorted := slices.Sorted(slices.Values(commands))
		repeated := len(slices.Compact(sorted)) != len(commands)
		listed := tc.listing != "" && slices.ContainsFunc(commands, func(c string) bool { return strings.Contains(c, tc.listing) })
		if len(commands) > tc.ceiling || repeated || listed {
			t.Fatalf("%s ran %d git commands, the ceiling is %d:\n%s", tc.name, len(commands), tc.ceiling, strings.Join(commands, "\n"))
		}
	}
}

// enrolledBeforeCommit is the real hook binding over a repository with a
// history whose HEAD holds no golaw.yml yet, the policy init --apply wrote
// enrolled and, when staged is true, added to the index.
func enrolledBeforeCommit(t *testing.T, staged bool) enrolled {
	t.Helper()
	b, home := testBinding(t)
	root := repository(t, false)
	gitRun(t, root, "add", "go.mod")
	gitRun(t, root, "commit", "-q", "-m", "build: start the module")
	if staged {
		gitRun(t, root, "add", "golaw.yml")
	}
	data := renderedPolicy(t)
	p, err := b.compile(data, now())
	if err != nil {
		t.Fatal(err)
	}
	if err = b.enroll(root, data, p, now()); err != nil {
		t.Fatal(err)
	}
	deps, err := b.hookDependencies(agentUnderTest(), now())
	if err != nil {
		t.Fatal(err)
	}
	return enrolled{deps: deps, root: root, home: home}
}

// TestHookAsksGitEachFactOnceBeforeTheFirstCommit counts the git commands
// of a Read and a Write while golaw.yml waits for its first commit: the
// index entry of the policy is listed once and the staged blob read once,
// for the policy in force and for the proposal alike, so the counts stay
// at those of a committed policy plus the one read of the staged blob.
func TestHookAsksGitEachFactOnceBeforeTheFirstCommit(t *testing.T) {
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	agent := agentUnderTest()
	for _, tc := range []struct {
		name          string
		staged        bool
		read, written int
	}{
		{"golaw.yml staged", true, 5, 6},
		{"golaw.yml untracked", false, 4, 5},
	} {
		e := enrolledBeforeCommit(t, tc.staged)
		for _, event := range []struct {
			tool    string
			input   map[string]any
			ceiling int
		}{
			{"Read", map[string]any{"file_path": filepath.Join(e.root, "go.mod")}, tc.read},
			{"Write", map[string]any{"file_path": filepath.Join(e.root, "notes.md"), "content": "one-two\n"}, tc.written},
		} {
			if err := os.WriteFile(trace, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			reply := hook.Run(t.Context(), agent.ID, strings.NewReader(eventAt(t, event.tool, e.root, event.input)), agent.Adapter, e.deps)
			if reply.ExitCode != 0 || len(reply.Stdout) != 0 || reply.Stderr != "" {
				t.Fatalf("%s, %s: exit %d, stdout %q, stderr %q", tc.name, event.tool, reply.ExitCode, reply.Stdout, reply.Stderr)
			}
			commands := gitCommandsOf(t, trace)
			sorted := slices.Sorted(slices.Values(commands))
			if len(commands) > event.ceiling || len(slices.Compact(sorted)) != len(commands) {
				t.Fatalf("%s, %s ran %d git commands, the ceiling is %d:\n%s", tc.name, event.tool, len(commands), event.ceiling, strings.Join(commands, "\n"))
			}
		}
	}
}
