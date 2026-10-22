package codex_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hooktest"
)

func TestShellRoutes(t *testing.T) {
	t.Parallel()
	for _, route := range []string{
		"bash", "heredoc-patch", "heredoc-patch-cd", "heredoc-cat",
	} {
		for _, tt := range []struct {
			side string
			want hook.Outcome
		}{
			{"red", hook.Deny},
			{"near", hook.Allow},
		} {
			t.Run(route+"/"+tt.side, func(t *testing.T) {
				t.Parallel()
				_, result := routeResult(t, route+"/"+tt.side)
				if result.Outcome != tt.want {
					t.Fatalf("want %s, got %s: %s",
						tt.want, result.Outcome, result.Reason)
				}
			})
		}
	}
}

func TestSharedShellFamily(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		command string
		want    hook.Outcome
	}{
		{"bypass", "git commit --no-verify -m x", hook.Deny},
		{"ordinary", "git commit -m x", hook.Allow},
		{"secret", "cat .env", hook.Deny},
		{"source", "cat credentials.go", hook.Allow},
		{"wrapped-bypass", "bash -lc 'git commit --no-verify -m x'", hook.Deny},
		{"wrapped-control", "bash -lc 'git status'", hook.Allow},
		{"push-default", "git push", hook.Allow},
		{"fetch", "git fetch", hook.Allow},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := commandPayload(t, "pre-bash-plain.json", tt.command)
			_, result := hooktest.Evaluate(t, "codex", adapter(), data, nil)
			if result.Outcome != tt.want {
				t.Fatalf("want %s, got %+v", tt.want, result)
			}
		})
	}
}
