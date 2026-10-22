package codex_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/hook/codex"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestEnvelope(t *testing.T) {
	t.Parallel()
	plain := corpus(t, "pre-bash-plain.json")
	tests := []struct {
		name string
		edit func(map[string]any)
		bad  bool
	}{
		{"plain", func(map[string]any) {}, false},
		{"future-field", func(m map[string]any) { m["future"] = true }, false},
		{"missing-turn", func(m map[string]any) { delete(m, "turn_id") }, true},
		{"empty-session", func(m map[string]any) { m["session_id"] = "" }, true},
		{"empty-use", func(m map[string]any) { m["tool_use_id"] = "" }, true},
		{"empty-cwd", func(m map[string]any) { m["cwd"] = "" }, true},
		{"array-input", func(m map[string]any) { m["tool_input"] = []any{} }, true},
		{"null-input", func(m map[string]any) { m["tool_input"] = nil }, true},
		{"foreign-conversation", func(m map[string]any) {
			m["conversation_id"] = nil
		}, true},
		{"foreign-generation", func(m map[string]any) {
			m["generation_id"] = "generation"
		}, true},
		{"foreign-roots", func(m map[string]any) {
			m["workspace_roots"] = []string{"/project"}
		}, true},
		{"foreign-version", func(m map[string]any) {
			m["cursor_version"] = nil
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parsed, err := codex.Parse(rewrite(t, plain, tt.edit))
			if tt.bad {
				if !errors.Is(err, model.ErrCannotJudge) {
					t.Fatalf("want cannot judge, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal("valid envelope was refused")
			}
			if parsed.Event.Agent != "codex" ||
				parsed.Event.SessionID != "session" ||
				parsed.Event.ToolUseID != "tool" ||
				parsed.Event.AgentID != "" {
				t.Fatalf("unexpected identity: %+v", parsed.Event)
			}
		})
	}
}

func TestOptionalAgentIdentity(t *testing.T) {
	t.Parallel()
	parsed, err := codex.Parse(corpus(t, "pre-bash-spawned.json"))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Event.AgentID != "agent" {
		t.Fatalf("agent identity = %q", parsed.Event.AgentID)
	}
}

func TestEnvelopeDispatch(t *testing.T) {
	t.Parallel()
	data := corpus(t, "pre-bash-plain.json")
	for _, tt := range []struct {
		agent string
		want  hook.Outcome
	}{
		{"codex", hook.Allow},
		{"claude", hook.Deny},
		{"cursor", hook.Deny},
	} {
		t.Run(tt.agent, func(t *testing.T) {
			t.Parallel()
			_, result := hooktest.Evaluate(t, tt.agent, adapter(), data, nil)
			if result.Outcome != tt.want {
				t.Fatalf("want %s, got %+v", tt.want, result)
			}
			if tt.want == hook.Deny && !result.CannotJudge {
				t.Fatal("agent mismatch must be cannot judge")
			}
		})
	}
}

func TestPlainShellRequest(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		command string
		agent   string
	}{
		{"pre-bash-plain.json", "go test ./...", ""},
		{"pre-bash-spawned.json", "git status --porcelain", "agent"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			request := decodeRequest(t, corpus(t, tt.name), nil)
			if request.Event.Shape != model.ShapeShell ||
				request.Event.Tool != "Bash" || request.Event.Shell == nil {
				t.Fatalf("unexpected shell event: %+v", request.Event)
			}
			if request.Event.Shell.Command != tt.command ||
				request.Event.Shell.Tool != "Bash" ||
				request.Event.AgentID != tt.agent {
				t.Fatalf("unexpected shell request: %+v", request)
			}
		})
	}
}

func TestUnknownCodexRoutesCannotJudge(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		field string
		value string
	}{
		{"tool_name", "spawn_agent"},
		{"tool_name", "mcp__server__tool"},
		{"tool_name", "Write"},
		{"tool_name", "Edit"},
		{"hook_event_name", "PermissionRequest"},
		{"hook_event_name", "SessionStart"},
		{"hook_event_name", "PostToolUseFailure"},
	} {
		t.Run(tt.value, func(t *testing.T) {
			t.Parallel()
			data := rewrite(t, corpus(t, "pre-bash-plain.json"), func(m map[string]any) {
				m[tt.field] = tt.value
			})
			_, result := hooktest.Evaluate(t, "codex", adapter(), data, nil)
			if !result.CannotJudge || result.Outcome == hook.Allow {
				t.Fatalf("unsupported route was allowed: %+v", result)
			}
		})
	}
}

func TestInstalledMatcherHasOnlySupportedRoutes(t *testing.T) {
	t.Parallel()
	data, err := assets.Read(agents.HookAsset("codex"))
	if err != nil {
		t.Fatal(err)
	}
	var definition struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string `json:"type"`
				Timeout int    `json:"timeout"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if decodeErr := json.Unmarshal(data, &definition); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if len(definition.Hooks) != 2 {
		t.Fatalf("events = %v", definition.Hooks)
	}
	for _, event := range []string{"PreToolUse", "PostToolUse"} {
		t.Run(event, func(t *testing.T) {
			t.Parallel()
			groups := definition.Hooks[event]
			if len(groups) != 1 || groups[0].Matcher != "^(Bash|apply_patch)$" ||
				len(groups[0].Hooks) != 1 {
				t.Fatalf("%s routes = %+v", event, groups)
			}
			handler := groups[0].Hooks[0]
			if handler.Type != "command" || handler.Timeout != 10 ||
				time.Duration(handler.Timeout)*time.Second <= 8*time.Second {
				t.Fatalf("%s handler = %+v", event, handler)
			}
		})
	}
}
