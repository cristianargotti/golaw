package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
)

// eventAt is a pre tool use event of a tool over a target from a session
// working directory of its own, the two locations the hook keeps apart.
func eventAt(t *testing.T, tool, cwd string, input map[string]any) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"session_id": "session", "agent_id": "agent", "tool_use_id": "use", "cwd": cwd,
		"hook_event_name": "PreToolUse", "tool_name": tool, "tool_input": input,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// enrolled is the real hook binding over a repository whose rendered
// policy is committed and enrolled, with the home of the binding and the
// agent's configuration directory under it.
type enrolled struct {
	deps               hook.Dependencies
	root, home, config string
}

func enrolledHook(t *testing.T) enrolled {
	t.Helper()
	b, home := testBinding(t)
	root := repository(t, true)
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
	return enrolled{deps: deps, root: root, home: home, config: agentUnderTest().ConfigDir(b.env, home)}
}

// TestHookJudgesTheRepositoryThatHoldsTheTarget drives the real binding
// through the agent's adapter: a dash written or a secret read in an
// enrolled repository from a session outside it is denied like one from
// inside; a dash written outside every repository from a session inside
// one, where the agent keeps its plans, is a positively unenrolled
// location; and a read outside every repository is still judged by the
// session's repository, so the credential stores of the person stay
// behind the secret boundary while an ordinary file there is readable.
func TestHookJudgesTheRepositoryThatHoldsTheTarget(t *testing.T) {
	t.Parallel()
	e := enrolledHook(t)
	root, home := e.root, e.home
	outside := t.TempDir()
	dash := func(target string) map[string]any {
		return map[string]any{"file_path": target, "content": "one \u2014 two\n"}
	}
	read := func(target string) map[string]any { return map[string]any{"file_path": target} }
	for _, tc := range []struct {
		name, tool, cwd string
		input           map[string]any
		denied          bool
	}{
		{"a dash into the repository from outside", "Write", outside, dash(filepath.Join(root, "notes.md")), true},
		{"a dash into the repository from inside", "Write", root, dash(filepath.Join(root, "notes.md")), true},
		{"a dash outside every repository from inside", "Write", root, dash(filepath.Join(e.config, "plans", "plan.md")), false},
		{"a secret of the repository from outside", "Read", outside, read(filepath.Join(root, ".env")), true},
		{"a credential store from inside", "Read", root, read(filepath.Join(home, ".ssh", "id_rsa")), true},
		{"an ordinary file outside from inside", "Read", root, read(filepath.Join(outside, "notes.md")), false},
	} {
		event := eventAt(t, tc.tool, tc.cwd, tc.input)
		agent := agentUnderTest()
		reply := hook.Run(t.Context(), agent.ID, strings.NewReader(event), agent.Adapter, e.deps)
		denied := strings.Contains(string(reply.Stdout), `"permissionDecision":"deny"`)
		if reply.ExitCode != 0 || denied != tc.denied || reply.Stderr != "" {
			t.Fatalf("%s: exit %d, stdout %q, stderr %q", tc.name, reply.ExitCode, reply.Stdout, reply.Stderr)
		}
	}
}

// links makes the symbolic links of a test, each a target and a name, or
// skips the test where the system has none.
func links(t *testing.T, pairs ...[2]string) {
	t.Helper()
	for _, pair := range pairs {
		if err := os.Symlink(pair[0], pair[1]); err != nil {
			t.Skipf("symbolic links are not available: %v", err)
		}
	}
}

// TestHookFollowsEveryLinkOfTheTarget drives the real binding with writes
// through links: one outside the repository to its policy file is denied
// from the repository and from outside, as is the policy file named with
// the leading space the host trims, a dash through a dangling link into
// the repository is denied while a clean write through it is allowed, and
// a link of the repository to a file outside stays with that repository,
// which cannot place it and refuses it.
func TestHookFollowsEveryLinkOfTheTarget(t *testing.T) {
	t.Parallel()
	e := enrolledHook(t)
	root, outside := e.root, t.TempDir()
	plain := filepath.Join(outside, "plain.md")
	if err := os.WriteFile(plain, []byte("plain\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	alias, dangling, out := filepath.Join(outside, "alias"), filepath.Join(outside, "dangling.md"), filepath.Join(root, "out.md")
	links(t, [2]string{filepath.Join(root, "golaw.yml"), alias}, [2]string{filepath.Join(root, "fresh.md"), dangling}, [2]string{plain, out})
	write := func(target, content string) map[string]any {
		return map[string]any{"file_path": target, "content": content}
	}
	for _, tc := range []struct {
		name, cwd    string
		input        map[string]any
		exit         int
		denied       bool
		reasonOrPath string
	}{
		{"the policy through an alias from the repository", root, write(alias, "x: 1\n"), 0, true, "golaw.yml"},
		{"the policy through an alias from outside", outside, write(alias, "x: 1\n"), 0, true, "golaw.yml"},
		{"the policy named with a leading space from outside", outside, write(" "+filepath.Join(root, "golaw.yml"), "x: 1\n"), 0, true, "golaw.yml"},
		{"a dash through a dangling alias from outside", outside, write(dangling, "one \u2014 two\n"), 0, true, "fresh.md"},
		{"a clean write through a dangling alias", outside, write(dangling, "one-two\n"), 0, false, ""},
		{"a link of the repository to a file outside", root, write(out, "one-two\n"), 2, false, "outside the repository"},
	} {
		agent := agentUnderTest()
		reply := hook.Run(t.Context(), agent.ID, strings.NewReader(eventAt(t, "Write", tc.cwd, tc.input)), agent.Adapter, e.deps)
		denied := strings.Contains(string(reply.Stdout), `"permissionDecision":"deny"`)
		said := string(reply.Stdout) + reply.Stderr
		if reply.ExitCode != tc.exit || denied != tc.denied || !strings.Contains(said, tc.reasonOrPath) {
			t.Fatalf("%s: exit %d, stdout %q, stderr %q", tc.name, reply.ExitCode, reply.Stdout, reply.Stderr)
		}
	}
}
