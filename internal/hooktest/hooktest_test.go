package hooktest_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/hooktest"
)

// first is the agent M1 built, which the helpers are proven with before the
// adapters of M4 exist; the registry names it, so this file names none.
func first() agents.Agent { return agents.All()[0] }

func write(content string) []byte {
	return []byte(`{"session_id":"session","tool_use_id":"tool","cwd":"/project","hook_event_name":"PreToolUse",` +
		`"tool_name":"Write","tool_input":{"file_path":"/project/notes.md","content":"` + content + `"}}`)
}

func TestPayloadSpellsTheRealRoot(t *testing.T) {
	t.Parallel()
	root := hooktest.Repository(t)
	got := string(hooktest.Payload(write("x"), root))
	if strings.Contains(got, hooktest.Root+"/") || !strings.Contains(got, filepath.ToSlash(root)+"/notes.md") {
		t.Fatalf("payload = %s", got)
	}
}

func TestEvaluateAndAnswerJudgeAPayload(t *testing.T) {
	t.Parallel()
	dash := "one" + string(rune(0x2014)) + "two"
	rows := []struct {
		name, content string
		want          hook.Outcome
	}{
		{"red dash", dash, hook.Deny},
		{"near hyphen", "one-two", hook.Allow},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			event, got := hooktest.Evaluate(t, first().ID, first().Adapter, write(row.content), map[string][]byte{"notes.md": []byte("safe\n")})
			if got.Outcome != row.want || event.Tool != "Write" {
				t.Fatalf("%+v", got)
			}
			reply := hooktest.Answer(t, first().ID, first().Adapter, write(row.content), nil)
			denied := strings.Contains(string(reply.Stdout), `"permissionDecision":"deny"`)
			if reply.ExitCode != 0 || denied != (row.want == hook.Deny) {
				t.Fatalf("%+v", reply)
			}
		})
	}
}

func TestProveRunsTheRoutesOfAManifest(t *testing.T) {
	t.Parallel()
	hooktest.Prove(t, filepath.Join("proof", "manifest.yml"), first().ID, first().Adapter)
}

// TestCorporaSpellTheRoot holds every payload of the corpora of the
// adapters to one JSON object whose directory is Root, so a decoder test,
// a proof fixture and the acceptance replay read the same files.
func TestCorporaSpellTheRoot(t *testing.T) {
	t.Parallel()
	corpora, err := filepath.Glob(filepath.Join("..", "hook", "*", "testdata", "payloads"))
	if err != nil || len(corpora) < 2 {
		t.Fatalf("corpora = %v, %v", corpora, err)
	}
	for _, corpus := range corpora {
		paths, err := filepath.Glob(filepath.Join(corpus, "*.json"))
		if err != nil || len(paths) < 6 {
			t.Fatalf("%s = %v, %v", corpus, paths, err)
		}
		for _, path := range paths {
			if reason := spellsRoot(path); reason != "" {
				t.Error(reason)
			}
		}
	}
}

// spellsRoot answers why a corpus payload breaks the convention, empty when
// it keeps it.
func spellsRoot(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return err.Error()
	}
	var payload struct {
		Cwd   string   `json:"cwd"`
		Roots []string `json:"workspace_roots"`
		Event string   `json:"hook_event_name"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || payload.Event == "" {
		return path + " is no hook payload"
	}
	if payload.Cwd != hooktest.Root && !slices.Equal(payload.Roots, []string{hooktest.Root}) {
		return path + " names no " + hooktest.Root
	}
	return ""
}
