package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// m4FakeCodex answers as golaw's Codex adapter answers the sample
// fixtures: a payload naming the marker RED is refused before the tool and
// reported after it, every other payload is allowed.
func m4FakeCodex(c command) result {
	switch {
	case !strings.Contains(c.input, "RED"):
		return result{}
	case strings.Contains(c.input, `"PostToolUse"`):
		return result{stdout: `{"decision":"block","reason":"` + sampleReason + `","systemMessage":"` + sampleHuman + `"}` + "\n"}
	default:
		return codexDeny()
	}
}

// m4SampleRoot is a module root holding the real corpus of an adapter and
// a sample proof manifest of two routes, a pre route and a post route with
// its pre event, each with a red fixture naming RED.
func m4SampleRoot(t *testing.T, corpus, manifest, pre, post string) string {
	t.Helper()
	root := t.TempDir()
	entries, err := os.ReadDir(filepath.Join(testRoot(t), filepath.FromSlash(corpus)))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		body, readErr := readFile(filepath.Join(testRoot(t), filepath.FromSlash(corpus), entry.Name()))
		if readErr != nil {
			t.Fatal(readErr)
		}
		checkedWrite(t, filepath.Join(root, filepath.FromSlash(corpus), entry.Name()), string(body))
	}
	proof := filepath.Join(root, filepath.Dir(filepath.FromSlash(manifest)))
	checkedWrite(t, filepath.Join(root, filepath.FromSlash(manifest)), "checks:\n"+
		"  - id: sample.shell\n    code: shell\n    red: testdata/shell/red\n    near: testdata/shell/near\n"+
		"  - id: sample.post\n    code: post\n    red: testdata/post/red\n    near: testdata/post/near\n")
	for _, side := range []string{"red", "near"} {
		marker := map[string]string{"red": "RED", "near": "near"}[side]
		m4WriteFixture(t, filepath.Join(proof, "testdata", "shell", side), map[string]string{
			"payload.json": strings.ReplaceAll(pre, "MARK", marker),
		})
		m4WriteFixture(t, filepath.Join(proof, "testdata", "post", side), map[string]string{
			"pre.json": strings.ReplaceAll(pre, "MARK", "pre"), "before/notes.md": "safe\n",
			"payload.json": strings.ReplaceAll(post, "MARK", marker), "notes.md": marker + "\n",
		})
	}
	return root
}

// m4CodexSample is a Linux replay of the real Codex corpus and two sample
// routes, answered by m4FakeCodex unless answer replaces it.
func m4CodexSample(t *testing.T, answer func(command) result) (*suite, *[]command) {
	t.Helper()
	s := m4ReplaySuite(t)
	s.root = m4SampleRoot(t, m4CodexCorpus, m4CodexManifest,
		`{"session_id":"session","turn_id":"turn","cwd":"/project","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"echo MARK"},"tool_use_id":"tool"}`,
		`{"session_id":"session","turn_id":"turn","cwd":"/project","hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"echo MARK"},"tool_response":"","tool_use_id":"tool"}`)
	seen := &[]command{}
	s.execute = m4Script(s, func(c command) result {
		*seen = append(*seen, c)
		return answer(c)
	})
	return s, seen
}

// m4RowNames are the names of rows, each with its status.
func m4RowNames(rows []row) []string {
	names := make([]string, 0, len(rows))
	for _, item := range rows {
		names = append(names, item.name+" "+item.status)
	}
	return names
}

func TestM4CodexRowsReplayTheCorpusAndTheRoutes(t *testing.T) {
	t.Parallel()
	s, seen := m4CodexSample(t, m4FakeCodex)
	rows := s.m4CodexRows(t.Context(), false)
	want := []string{
		"post-apply-patch PASS", "post-bash PASS", "pre-apply-patch-add PASS", "pre-apply-patch-update-move-delete PASS",
		"pre-bash-heredoc-patch-cd PASS", "pre-bash-heredoc-patch PASS", "pre-bash-plain PASS", "pre-bash-spawned PASS",
		"shell-red PASS", "shell-near PASS", "post-red PASS", "post-near PASS",
	}
	if got := m4RowNames(rows); !slices.Equal(got, want) {
		t.Fatalf("rows = %q\n%+v", got, rows)
	}
	var derived []string
	for _, c := range *seen {
		if strings.Contains(c.input, "notes/out.txt") {
			derived = append(derived, c.input)
		}
		if strings.Contains(c.input, "internal/app/old.go") {
			if _, err := os.Stat(filepath.Join(c.dir, "internal", "app", "old.go")); err != nil {
				t.Fatalf("the update payload ran before its file existed: %v", err)
			}
		}
	}
	if len(derived) != 2 || !strings.Contains(derived[0], `"PreToolUse"`) || strings.Contains(derived[0], "tool_response") ||
		!strings.Contains(derived[1], `"PostToolUse"`) {
		t.Fatalf("the post-bash replay sent %q", derived)
	}
}

func TestM4CodexRowsFailWhatTheContractRefuses(t *testing.T) {
	t.Parallel()
	s, _ := m4CodexSample(t, func(command) result { return result{} })
	rows := s.m4CodexRows(t.Context(), false)
	for _, item := range rows {
		red := strings.HasSuffix(item.name, "-red")
		if red && (item.status != statusFail || !item.required) || !red && item.status != statusPass {
			t.Errorf("an adapter that allows everything: %+v", item)
		}
	}
	checkedWrite(t, filepath.Join(s.root, filepath.FromSlash(m4CodexCorpus), "extra.json"), "{}")
	if err := os.Remove(filepath.Join(s.root, filepath.FromSlash(m4CodexManifest))); err != nil {
		t.Fatal(err)
	}
	got := m4RowNames(s.m4CodexRows(t.Context(), false))
	for _, name := range []string{"extra FAIL", "corpus FAIL", "manifest FAIL"} {
		if !slices.Contains(got, name) {
			t.Errorf("rows %q lack %q", got, name)
		}
	}
	s.root = t.TempDir()
	if got := m4RowNames(s.m4CodexRows(t.Context(), false)); !slices.Equal(got, []string{"corpus FAIL", "manifest FAIL"}) {
		t.Fatalf("an absent corpus and manifest = %q", got)
	}
}

func TestM4DerivedPreDropsThePostFields(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ agent, post, event, gone string }{
		{agentCodex, `{"hook_event_name":"PostToolUse","tool_response":"","tool_use_id":"tool"}`, `"PreToolUse"`, "tool_response"},
		{agentCursor, `{"hook_event_name":"postToolUseFailure","tool_output":"x","duration":1,"error_message":"e","failure_type":"error","is_interrupt":false,"tool_use_id":"tool"}`, `"preToolUse"`, "error_message"},
	} {
		pre, err := m4DerivedPre(tc.agent, []byte(tc.post))
		if err != nil || !strings.Contains(string(pre), tc.event) || strings.Contains(string(pre), tc.gone) || !strings.Contains(string(pre), `"tool_use_id":"tool"`) {
			t.Errorf("%s: %s, %v", tc.agent, pre, err)
		}
	}
	if _, err := m4DerivedPre(agentCodex, []byte("{")); err == nil {
		t.Fatal("an undecodable post payload passed")
	}
	if m4RouteWant(agentCursor, cursorPostTool, true) != m4Block || m4RouteWant(agentCursor, cursorReadFile, true) != m4Deny ||
		m4RouteWant(agentCodex, eventPost, false) != m4Allow {
		t.Fatal("the answers of the routes are wrong")
	}
}
