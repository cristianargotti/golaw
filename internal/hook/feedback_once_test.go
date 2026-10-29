package hook_test

import (
	"encoding/json"
	"maps"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hooktest"
)

// again is the payload with the fields given changed.
func again(t *testing.T, payload []byte, fields map[string]any) []byte {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(payload, &document); err != nil {
		t.Fatal(err)
	}
	maps.Copy(document, fields)
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// session judges the payloads in order over one repository and one state,
// each over its own files, and answers the results.
func session(t *testing.T, rec *recorder, steps ...step) []hook.Result {
	t.Helper()
	root := hooktest.Repository(t)
	var state hook.Store
	results := make([]hook.Result, 0, len(steps))
	for i, step := range steps {
		deps := hooktest.Dependencies(t, root, step.files)
		if i == 0 {
			state = deps.State
		}
		deps.State, deps.Tools = state, rec.tools
		_, result := hook.Evaluate(t.Context(), "claude", hooktest.Payload(step.payload, root), claudeAdapter(), deps)
		results = append(results, result)
	}
	return results
}

// step is one event of a session: its payload and the files the
// repository holds when it is judged.
type step struct {
	payload []byte
	files   map[string][]byte
}

func TestTheFeedbackOfAFileComesOncePerSessionAndContent(t *testing.T) {
	t.Parallel()
	first := "FROM alpine@" + digest + "\n"
	edited := "# base\nFROM alpine@" + digest + "\n"
	post := written(t, "PostToolUse", "Dockerfile", first)
	results := session(t, &recorder{},
		step{post, map[string][]byte{"Dockerfile": []byte(first)}},
		step{again(t, post, map[string]any{"tool_use_id": "tool-2"}), map[string][]byte{"Dockerfile": []byte(first)}},
		step{again(t, post, map[string]any{"tool_use_id": "tool-3"}), map[string][]byte{"Dockerfile": []byte(edited)}},
		step{again(t, post, map[string]any{"tool_use_id": "tool-4", "session_id": "other"}), map[string][]byte{"Dockerfile": []byte(edited)}},
	)
	counts := []int{len(results[0].Lines), len(results[1].Lines), len(results[2].Lines), len(results[3].Lines)}
	if counts[0] != 1 || counts[1] != 0 || counts[2] != 1 || counts[3] != 1 {
		t.Fatalf("lines per event = %v, want the first, the changed content and the other session only", counts)
	}
	if results[1].Outcome != hook.Allow || results[1].Human != "" {
		t.Fatalf("a repeated event = %+v, want a plain allow", results[1])
	}
}

func TestTheMissingToolNoticeComesOncePerSessionAndTool(t *testing.T) {
	t.Parallel()
	rec := &recorder{report: hook.ToolReport{
		Missing: []string{"ruff 0.17.2 is not installed: a.py was judged by the native checks only; run golaw setup --surface, or the pipeline judges the rest."},
		Person:  []string{"ruff 0.17.2 não está instalado: o golaw julgou a.py só com as verificações nativas; rode golaw setup --surface ou deixe o resto para o pipeline."},
	}}
	first := written(t, "PostToolUse", "a.py", "x = 1\n")
	second := again(t, written(t, "PostToolUse", "b.py", "y = 1\n"), map[string]any{"tool_use_id": "tool-2"})
	results := session(t, rec,
		step{first, map[string][]byte{"a.py": []byte("x = 1\n")}},
		step{second, map[string][]byte{"b.py": []byte("y = 1\n")}},
	)
	if !strings.Contains(results[0].Notice, "ruff 0.17.2 is not installed") || strings.Contains(results[1].Notice, "is not installed") {
		t.Fatalf("notices = %q and %q, want the first only", results[0].Notice, results[1].Notice)
	}
}
