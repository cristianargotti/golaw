package cursor_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestCursorProtocolAlwaysAnswers(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		data []byte
		want string
	}{
		{"red empty", nil, "deny"},
		{"red malformed", []byte("{"), "deny"},
		{"red null", []byte("null"), "deny"},
		{"red foreign", curBytes(t, "proof/testdata/envelope/red/payload.json"), "deny"},
		{"red shell", curBytes(t, "proof/testdata/shell/red/payload.json"), "deny"},
		{"near shell", curBytes(t, "proof/testdata/shell/near/payload.json"), "allow"},
		{"near write", curBytes(t, "proof/testdata/write-capture/near/payload.json"), "allow"},
		{"near delete", curBytes(t, "proof/testdata/delete-capture/near/payload.json"), "allow"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			document := curDocument(t, hooktest.Answer(t, "cursor", curAdapter(), test.data, nil))
			if document["permission"] != test.want {
				t.Fatalf("permission = %#v", document)
			}
		})
	}
}

func TestCursorOutsideScopePrintsAllow(t *testing.T) {
	t.Parallel()
	root := hooktest.Repository(t)
	dependencies := hooktest.Dependencies(t, root, nil)
	dependencies.Enroll = func(context.Context, string, string) (hook.Repository, error) {
		return hook.Repository{
			Root: root, Identity: root,
			Enrollment: model.Enrollment{State: model.EnrollmentNone},
		}, nil
	}
	data := curBytes(t, "proof/testdata/shell/red/payload.json")
	reply := hook.Run(t.Context(), "cursor",
		bytes.NewReader(hooktest.Payload(data, root)), curAdapter(), dependencies)
	if document := curDocument(t, reply); document["permission"] != "allow" {
		t.Fatalf("unenrolled reply = %#v", document)
	}
}

func TestCursorInternalFailurePrintsDeny(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"deadline", "panic"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			root := hooktest.Repository(t)
			dependencies := hooktest.Dependencies(t, root, nil)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if failure == "deadline" {
				cancel()
			} else {
				dependencies.Policy = func(context.Context, hook.Repository) (*model.EffectivePolicy, error) {
					panic("probe failure")
				}
			}
			data := curBytes(t, "proof/testdata/shell/near/payload.json")
			reply := hook.Run(ctx, "cursor",
				bytes.NewReader(hooktest.Payload(data, root)), curAdapter(), dependencies)
			document := curDocument(t, reply)
			if document["permission"] != "deny" || document["agent_message"] == "" {
				t.Fatalf("failure reply = %#v", document)
			}
		})
	}
}
