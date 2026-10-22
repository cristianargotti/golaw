package cursor

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

var errShrinkInput = errors.New("input unavailable")

// TestShrinkKeepsTheKeysAndDropsTheReadContent pins the copy itself: the
// content beside the event dropped whatever its length, a nested content
// kept, a number past the shorter bound kept as 0, a key past the longer
// bound elided with its value kept, and every other value as it came.
func TestShrinkKeepsTheKeysAndDropsTheReadContent(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("k", keptField+1)
	digits := strings.Repeat("7", keptText+1)
	input := `{"content":"PRIVATE","tool_input":{"content":"kept","n":-1.5e+3},` +
		`"n":` + digits + `,"` + long + `":[true,false,null],"attachments":["x"]} `
	want := `{"content":` + elision + `,"tool_input":{"content":"kept","n":-1.5e+3},` +
		`"n":0,` + elision + `:[true,false,null],"attachments":["x"]}`
	got, err := shrink(strings.NewReader(input))
	if err != nil || string(got) != want {
		t.Fatalf("copy = %.200q, %v", got, err)
	}
	if root, err := shrink(strings.NewReader("12")); err != nil || string(root) != "12" {
		t.Fatalf("a number that ends the input = %q, %v", root, err)
	}
}

func TestShrinkRefusesAnInputItCannotRead(t *testing.T) {
	t.Parallel()
	failing := func(text string) io.Reader {
		return io.MultiReader(strings.NewReader(text), iotest.ErrReader(errShrinkInput))
	}
	for _, test := range []struct {
		name  string
		input io.Reader
	}{
		{"red failure inside a number", failing(`{"a":12`)},
		{"red failure inside a literal", failing(`{"a":tr`)},
		{"red failure inside a key", failing(`{"ab`)},
		{"red end inside a literal", strings.NewReader(`{"a":tr`)},
		{"red copy over the bound", strings.NewReader("[" + strings.Repeat("0,", hook.MaxInputBytes/2) + "0]")},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := shrink(test.input); !errors.Is(err, model.ErrCannotJudge) {
				t.Fatalf("expected cannot judge, got %v", err)
			}
		})
	}
}

// TestOversizedRefusesWhatTheCopyCannotJudge holds an input over the bound
// whose copy is no Cursor event, whose header field was elided or whose
// route refuses its field to cannot judge, the answer of the same input
// within the bound or of a field the hook could not keep.
func TestOversizedRefusesWhatTheCopyCannotJudge(t *testing.T) {
	t.Parallel()
	shell := `{"hook_event_name":"beforeShellExecution","conversation_id":"c","workspace_roots":["/r"],"command":"git status","cwd":`
	read := `{"hook_event_name":"beforeReadFile","conversation_id":"c","workspace_roots":["/r"],"file_path":7}`
	for _, test := range []struct{ name, input string }{
		{"red no cursor event", `{"hook_event_name":"PreToolUse"}`},
		{"red elided working directory", shell + `"/` + strings.Repeat("d", keptField) + `"}`},
		{"red read without a name", read},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			parsed, err := Oversized(strings.NewReader(test.input))
			if err == nil {
				_, err = parsed.Decode(hook.Base{Root: t.TempDir()}, nil)
			}
			if !errors.Is(err, model.ErrCannotJudge) {
				t.Fatalf("expected cannot judge, got %v", err)
			}
		})
	}
	parsed, err := Oversized(bytes.NewReader([]byte(shell + `"/r"}`)))
	if err != nil || parsed.Event.Cwd != "/r" {
		t.Fatalf("kept working directory = %+v, %v", parsed.Event, err)
	}
}
