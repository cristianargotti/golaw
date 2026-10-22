package codex_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestMalformedInputAlwaysExplainsItself(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"", "{", "null", "[]", "{}{}", `{"tool_input":`} {
		reply := hooktest.Answer(t, "codex", adapter(), []byte(input), nil)
		if reply.ExitCode != 0 || !bytes.Contains(reply.Stdout, []byte("\"permissionDecision\":\"deny\"")) ||
			strings.TrimSpace(reply.Stderr) == "" {
			t.Fatalf("input %q produced %+v", input, reply)
		}
	}
}

func TestInternalDeadline(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		seed string
		want string
	}{
		{"pre", "pre-bash-plain.json", "\"permissionDecision\":\"deny\""},
		{"post", "post-bash.json", "additionalContext"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := hooktest.Repository(t)
			dependencies := hooktest.Dependencies(t, root, nil)
			dependencies.Budget = time.Millisecond
			dependencies.Policy = func(ctx context.Context, _ hook.Repository) (*model.EffectivePolicy, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			}
			data := hooktest.Payload(corpus(t, tt.seed), root)
			reply := hook.Run(t.Context(), "codex",
				bytes.NewReader(data), adapter(), dependencies)
			if reply.ExitCode != 0 {
				t.Fatalf("exit = %d, want 0", reply.ExitCode)
			}
			if !bytes.Contains(reply.Stdout, []byte(tt.want)) {
				t.Fatalf("stdout %s lacks %s", reply.Stdout, tt.want)
			}
			if tt.name == "pre" && strings.TrimSpace(reply.Stderr) == "" {
				t.Fatal("deadline has no blocking reason")
			}
		})
	}
}

func TestInvalidBudgetAndPanicCannotFailOpen(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		budget time.Duration
		fault  bool
	}{
		{"negative", -time.Second, false},
		{"host-boundary", 10 * time.Second, false},
		{"panic", time.Second, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := hooktest.Repository(t)
			dependencies := hooktest.Dependencies(t, root, nil)
			dependencies.Budget = tt.budget
			if tt.fault {
				dependencies.Policy = func(context.Context, hook.Repository) (*model.EffectivePolicy, error) {
					panic("test fault")
				}
			}
			data := hooktest.Payload(corpus(t, "pre-bash-plain.json"), root)
			reply := hook.Run(t.Context(), "codex",
				bytes.NewReader(data), adapter(), dependencies)
			if reply.ExitCode != 0 || !bytes.Contains(reply.Stdout, []byte("\"permissionDecision\":\"deny\"")) ||
				strings.TrimSpace(reply.Stderr) == "" {
				t.Fatalf("fault was not explained: %+v", reply)
			}
		})
	}
}

func TestUnenrolledLocationAllows(t *testing.T) {
	t.Parallel()
	root := hooktest.Repository(t)
	dependencies := hooktest.Dependencies(t, root, nil)
	dependencies.Enroll = func(context.Context, string, string) (hook.Repository, error) {
		return hook.Repository{
			Root: root, Enrollment: model.Enrollment{State: model.EnrollmentNone},
		}, nil
	}
	dependencies.Policy = func(context.Context, hook.Repository) (*model.EffectivePolicy, error) {
		t.Fatal("unenrolled event read policy")
		return nil, model.ErrCannotJudge
	}
	data := commandPayload(t, "pre-bash-plain.json", "cat .env")
	reply := hook.Run(t.Context(), "codex",
		bytes.NewReader(hooktest.Payload(data, root)), adapter(), dependencies)
	if reply.ExitCode != 0 || len(reply.Stdout) != 0 || reply.Stderr != "" {
		t.Fatalf("unenrolled location refused: %+v", reply)
	}
}
