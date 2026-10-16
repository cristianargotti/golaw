package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
)

// hostWrite is a write event in the field order the host writes, the tool
// input after the header and its path before its content, so an input over
// the bound keeps its header; first puts the content first instead.
func hostWrite(t *testing.T, cwd, path, content string, first bool) string {
	t.Helper()
	quoted := func(text string) string {
		data, err := json.Marshal(text)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	input := fmt.Sprintf(`{"file_path":%s,"content":%s}`, quoted(path), quoted(content))
	if first {
		input = fmt.Sprintf(`{"content":%s,"file_path":%s}`, quoted(content), quoted(path))
	}
	return fmt.Sprintf(`{"session_id":"session","cwd":%s,"hook_event_name":"PreToolUse","tool_name":"Write","tool_input":%s,"tool_use_id":"bounds"}`,
		quoted(cwd), input)
}

// noticeOf decodes the answer of the hook: the additional context the
// agent reads and the message the person reads.
func noticeOf(t *testing.T, reply hook.Reply) (string, string) {
	t.Helper()
	var answer struct {
		Specific struct {
			Context string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
		Human string `json:"systemMessage"`
	}
	if err := json.Unmarshal(reply.Stdout, &answer); err != nil {
		t.Fatalf("answer %q: %v", reply.Stdout, err)
	}
	return answer.Specific.Context, answer.Human
}

// TestHookJudgesAWriteUpToTheReadBound drives writes past the earlier
// input bound through the real binding: a write whose event is past 1 MiB
// is judged whole, a write over 8 MiB is allowed with a notice naming the
// file it did not measure, whether its path comes before or after its
// content, and the policy file stays protected at any size.
func TestHookJudgesAWriteUpToTheReadBound(t *testing.T) { //nolint:paralleltest // an event of 8 MiB runs alone, so its deadline measures the hook, not the tests beside it
	e := enrolledHook(t)
	agent := agentUnderTest()
	run := func(path, content string) hook.Reply {
		event := hostWrite(t, e.root, filepath.Join(e.root, path), content, false)
		return hook.Run(t.Context(), agent.ID, strings.NewReader(event), agent.Adapter, e.deps)
	}
	// White space between the fields carries the event past the earlier
	// bound of 1 MiB, so the rules read a content of a moment.
	padded := strings.Replace(hostWrite(t, e.root, filepath.Join(e.root, "notes.md"), "one \u2014 two\n", false),
		`,"tool_use_id"`, strings.Repeat(" ", 1<<20)+`,"tool_use_id"`, 1)
	judged := hook.Run(t.Context(), agent.ID, strings.NewReader(padded), agent.Adapter, e.deps)
	if judged.ExitCode != 0 || !strings.Contains(string(judged.Stdout), `"permissionDecision":"deny"`) || !strings.Contains(string(judged.Stdout), "text.dashes") {
		t.Fatalf("a write whose event is past 1 MiB: %+v", judged)
	}
	over := run("data.md", strings.Repeat("a", hook.MaxInputBytes))
	agentText, humanText := noticeOf(t, over)
	if over.ExitCode != 0 || strings.Contains(string(over.Stdout), "permissionDecision") ||
		!strings.Contains(agentText, "data.md") || !strings.Contains(agentText, "the pipeline judges") || !strings.Contains(humanText, "data.md") {
		t.Fatalf("a write over the bound: %+v", over)
	}
	policy := run("golaw.yml", strings.Repeat("a", hook.MaxInputBytes))
	if policy.ExitCode != 0 || !strings.Contains(string(policy.Stdout), `"permissionDecision":"deny"`) {
		t.Fatalf("the policy file over the bound: %+v", policy)
	}
	late := hostWrite(t, e.root, filepath.Join(e.root, "data.md"), strings.Repeat("a", hook.MaxInputBytes), true)
	reply := hook.Run(t.Context(), agent.ID, strings.NewReader(late), agent.Adapter, e.deps)
	if agentText, _ := noticeOf(t, reply); reply.ExitCode != 0 || !strings.Contains(agentText, "data.md") {
		t.Fatalf("a path after the content over the bound: %+v", reply)
	}
}

// TestHookPostBoundsAreNotices drives post events through the real binding
// whose detection meets a bound or no baseline: a file a command wrote over
// the read bound and a post event without its pre event are each allowed
// with a notice the agent reads, naming the file when there is one.
func TestHookPostBoundsAreNotices(t *testing.T) { //nolint:paralleltest // an event of 8 MiB runs alone, so its deadline measures the hook, not the tests beside it
	e := enrolledHook(t)
	agent := agentUnderTest()
	shell := func(name, use string) hook.Reply {
		event := strings.Replace(shellEventAt(t, name, e.root, "true"), `"tool_use_id":"shell"`, `"tool_use_id":"`+use+`"`, 1)
		return hook.Run(t.Context(), agent.ID, strings.NewReader(event), agent.Adapter, e.deps)
	}
	if pre := shell("PreToolUse", "large"); pre.ExitCode != 0 || len(pre.Stdout) != 0 {
		t.Fatalf("pre: %+v", pre)
	}
	writeFiles(t, e.root, map[string]string{"dump.sql": strings.Repeat("a", hook.MaxInputBytes+1)})
	for _, tc := range []struct{ use, names string }{{"large", "dump.sql"}, {"orphan", "pipeline"}} {
		reply := shell("PostToolUse", tc.use)
		agentText, humanText := noticeOf(t, reply)
		if reply.ExitCode != 0 || strings.Contains(string(reply.Stdout), `"decision":"block"`) ||
			!strings.Contains(agentText, tc.names) || !strings.Contains(agentText, "the pipeline judges") || humanText == "" {
			t.Fatalf("%s: %+v", tc.use, reply)
		}
	}
}
