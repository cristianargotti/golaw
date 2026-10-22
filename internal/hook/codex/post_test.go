package codex_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestPostRoutes(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"post-apply-patch", "post-bash"} {
		for _, tt := range []struct {
			side string
			want hook.Outcome
		}{
			{"red", hook.Feedback},
			{"near", hook.Allow},
		} {
			t.Run(route+"/"+tt.side, func(t *testing.T) {
				t.Parallel()
				event, result := routeResult(t, route+"/"+tt.side)
				if result.Outcome != tt.want {
					t.Fatalf("want %s, got %s: %s",
						tt.want, result.Outcome, result.Reason)
				}
				if event.Shape != model.ShapePost || event.Post == nil ||
					event.Post.Failed {
					t.Fatalf("incorrect post identity: %+v", event)
				}
			})
		}
	}
}

func TestPostPatchHints(t *testing.T) {
	t.Parallel()
	text := patch("*** Update File: old.md\n*** Move to: new.md\n" +
		"@@\n-old\n+new\n*** Delete File: gone.md\n")
	data := commandPayload(t, "post-apply-patch.json", text)
	request := decodeRequest(t, data, nil)
	want := []string{"gone.md", "new.md", "old.md"}
	if !reflect.DeepEqual(request.Hints, want) {
		t.Fatalf("hints = %v, want %v", request.Hints, want)
	}
	if request.Event.Shape != model.ShapePost {
		t.Fatal("patch post lost its shape")
	}
}

func TestPostResponseAndIdentity(t *testing.T) {
	t.Parallel()
	for _, response := range []any{"", "exit 1", nil, map[string]any{"output": "x"}} {
		data := rewrite(t, corpus(t, "post-bash.json"), func(m map[string]any) {
			m["tool_response"] = response
			m["agent_id"] = "agent"
		})
		post := decodeRequest(t, data, nil)
		preData := rewrite(t, corpus(t, "pre-bash-plain.json"), func(m map[string]any) {
			m["agent_id"] = "agent"
		})
		pre := decodeRequest(t, preData, nil)
		preKey, preErr := hook.StateKey("repository", pre.Event)
		postKey, postErr := hook.StateKey("repository", post.Event)
		if preErr != nil || postErr != nil || preKey != postKey {
			t.Fatalf("state identity changed: %v %v", preErr, postErr)
		}
		if post.Event.Post.Failed {
			t.Fatal("response text was interpreted as an exit status")
		}
	}
}

func TestPostMissingBaselineIsNotice(t *testing.T) {
	t.Parallel()
	data := corpus(t, "post-bash.json")
	_, result := hooktest.Evaluate(t, "codex", adapter(), data, nil)
	if result.Outcome != hook.Allow || result.Notice == "" {
		t.Fatalf("missing baseline must be a notice: %+v", result)
	}
}

func TestPostMissingResponseCannotJudge(t *testing.T) {
	t.Parallel()
	data := rewrite(t, corpus(t, "post-bash.json"), func(m map[string]any) {
		delete(m, "tool_response")
	})
	_, result := hooktest.Evaluate(t, "codex", adapter(), data, nil)
	if !result.CannotJudge || result.Outcome == hook.Allow {
		t.Fatalf("missing response was allowed: %+v", result)
	}
	reply := hooktest.Answer(t, "codex", adapter(), data, nil)
	if reply.ExitCode != 2 || strings.TrimSpace(reply.Stderr) == "" {
		t.Fatalf("missing response can fail open: %+v", reply)
	}
}

func TestPostCorpus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		tool     string
		hints    []string
		response string
	}{
		{
			name: "post-apply-patch.json", tool: "apply_patch",
			hints:    []string{"notes/hello.txt"},
			response: "Success. Updated the following files:\nA notes/hello.txt\n",
		},
		{name: "post-bash.json", tool: "Bash", response: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			request := decodeRequest(t, corpus(t, tt.name), nil)
			if request.Event.Shape != model.ShapePost ||
				request.Event.Tool != tt.tool || request.Event.Post == nil {
				t.Fatalf("incorrect post event: %+v", request.Event)
			}
			if !reflect.DeepEqual(request.Hints, tt.hints) ||
				request.Event.Post.Response != tt.response || request.Event.Post.Failed {
				t.Fatalf("incorrect post request: %+v", request)
			}
		})
	}
}
