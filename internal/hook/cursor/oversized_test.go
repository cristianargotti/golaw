package cursor_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/cursor"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestCursorOversizedRead(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		path string
		want hook.Outcome
	}{
		{"red secret", ".env", hook.Deny},
		{"near source", "main.go", hook.Allow},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			curCheckOversizedRead(t, test.path, test.want)
		})
	}
}

func curCheckOversizedRead(t *testing.T, path string, want hook.Outcome) {
	t.Helper()
	root := hooktest.Repository(t)
	data := curBytes(t, "proof/testdata/read/near/payload.json")
	data = curPatch(t, data, map[string]any{
		"file_path": "/project/" + path,
		"content":   strings.Repeat("x", hook.MaxInputBytes+1),
	})
	data = hooktest.Payload(data, root)
	parsed, err := cursor.Oversized(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	request, err := parsed.Decode(hook.Base{Root: root}, proof.NewTree(nil))
	if err != nil || request.Event.Read == nil ||
		request.Event.Read.Path != path || len(request.Content) != 0 {
		t.Fatalf("oversized read = %+v, %v", request, err)
	}
	adapter := curAdapter()
	adapter.Parse = func([]byte) (hook.Parsed, error) { return parsed, nil }
	_, result := hook.Evaluate(
		t.Context(), "cursor", nil, adapter, hooktest.Dependencies(t, root, nil),
	)
	if result.Outcome != want {
		t.Fatalf("result = %+v", result)
	}
}

func TestCursorOversizedRejectsIncompleteInput(t *testing.T) {
	t.Parallel()
	base := curBytes(t, "proof/testdata/read/near/payload.json")
	large := curPatch(t, base, map[string]any{
		"content": strings.Repeat("x", hook.MaxInputBytes+1),
	})
	tests := []struct {
		name string
		data []byte
		bad  bool
	}{
		{"red truncated", large[:len(large)-1], true},
		{"red second object", append(bytes.Clone(large), []byte("{}")...), true},
		{"red invalid escape", []byte(`{"content":"\q"}`), true},
		{"near complete", large, false},
		{"near trailing whitespace", append(bytes.Clone(large), []byte(" \t\r\n")...), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := cursor.Oversized(bytes.NewReader(test.data))
			if test.bad && !errors.Is(err, model.ErrCannotJudge) {
				t.Fatalf("expected cannot judge, got %v", err)
			}
			if !test.bad && err != nil {
				t.Fatal(err)
			}
		})
	}
}
