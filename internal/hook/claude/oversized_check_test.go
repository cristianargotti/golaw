package claude_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/claude"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// overBound is a content of more bytes than the hook reads.
func overBound() string { return strings.Repeat("a", model.MaxReadBytes+1) }

// padded carries an event over the bound in a field of the envelope the
// adapter does not read.
func padded(event string) string {
	return strings.Replace(event, `"transcript_path":""`, `"transcript_path":"`+overBound()+`"`, 1)
}

// TestOversizedChecksTheJSONAsTheDecoderDoes refuses an input over the
// bound the decoder of an event within it refuses: a raw newline, an
// unknown escape, a raw control character or a bad unicode escape inside a
// string, a number, a literal or a structure the grammar refuses, and a
// structure deeper than the decoder reads; the same event made valid, an
// escape of every kind and bytes that are not UTF-8 included, is read.
func TestOversizedChecksTheJSONAsTheDecoderDoes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := fmt.Sprintf("%q", filepath.Join(root, "big.md"))
	write := func(content, extra string) string {
		return ordered("PreToolUse", root, `{"file_path":`+path+`,"content":"`+content+`"`+extra+`}`)
	}
	big := overBound()
	for name, input := range map[string]string{
		"a raw newline":                      write(big+"\n", ""),
		"an unknown escape":                  write(big+`\q`, ""),
		"a raw control character":            write(big+"\x01", ""),
		"a unicode escape of three digits":   write(big+`\u12g4`, ""),
		"a number with a leading zero":       write(big, `,"n":01`),
		"a number without its fraction":      write(big, `,"n":1.`),
		"a minus alone":                      write(big, `,"n":-`),
		"a literal cut short":                write(big, `,"n":tru`),
		"a trailing comma in an array":       write(big, `,"n":[1,]`),
		"a bracket that closes another kind": write(big, `,"n":[1}`),
		"a structure past the decoder depth": write(big, `,"n":`+strings.Repeat("[", 10001)+strings.Repeat("]", 10001)),
		"a key that is no string":            write(big, `,7:1`),
	} {
		if _, err := claude.Oversized(strings.NewReader(input)); !errors.Is(err, model.ErrCannotJudge) {
			t.Errorf("%s: %v", name, err)
		}
	}
	valid := write(big+`\"\\\/\b\f\n\r\té`+"\xff\x7f", `,"n":[-0.5e+3,0,10E2,true,false,null,{},[]]`)
	if parsed, err := claude.Oversized(strings.NewReader(valid)); err != nil || parsed.Event.Target != filepath.Join(root, "big.md") {
		t.Fatalf("a valid event = %+v, %v", parsed.Event, err)
	}
}

// TestOversizedKeepsTheChecksOfTheDecoders answers an event over the bound
// with what the decoder of an event within it answers, but for the bytes
// it does not hold: NotebookEdit is denied for compatibility, an unknown
// field, a missing content, mixed edit schemas, a failure event of a write
// tool, a post event without its response and a pre event of another tool
// are refused, and a write is the candidate of its path with the gap of the
// input, while a post event answers the request of any post event.
func TestOversizedKeepsTheChecksOfTheDecoders(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "big.md")
	quoted, big := fmt.Sprintf("%q", path), overBound()
	tree := proof.NewTree(map[string][]byte{"big.md": []byte("old")})
	decode := func(input string) (hook.Request, error) {
		parsed, err := claude.Oversized(strings.NewReader(input))
		if err != nil {
			return hook.Request{}, fmt.Errorf("parse: %w", err)
		}
		return parsed.Decode(hook.Base{Root: root}, tree)
	}
	for name, input := range map[string]string{
		"an unknown field":        ordered("PreToolUse", root, `{"file_path":`+quoted+`,"content":"`+big+`","unknown":1}`),
		"no content":              padded(ordered("PreToolUse", root, `{"file_path":`+quoted+`}`)),
		"a content that is null":  padded(ordered("PreToolUse", root, `{"file_path":`+quoted+`,"content":null}`)),
		"a failure of a write":    strings.Replace(ordered("PostToolUseFailure", root, `{"file_path":`+quoted+`,"content":"`+big+`"}`), `"tool_use_id"`, `"error":"failed","tool_use_id"`, 1),
		"a post without response": ordered("PostToolUse", root, `{"file_path":`+quoted+`,"content":"`+big+`"}`),
		"a read over the bound":   strings.Replace(ordered("PreToolUse", root, `{"file_path":`+quoted+`,"x":"`+big+`"}`), `"tool_name":"Write"`, `"tool_name":"Read"`, 1),
		"mixed edit schemas": strings.Replace(ordered("PreToolUse", root, `{"file_path":`+quoted+`,"old_string":"old","new_string":"`+big+`","edits":[{"old_string":"o","new_string":"n"}]}`),
			`"tool_name":"Write"`, `"tool_name":"Edit"`, 1),
	} {
		if got, err := decode(input); !errors.Is(err, model.ErrCannotJudge) {
			t.Errorf("%s: %+v, %v", name, got, err)
		}
	}
	notebook := strings.Replace(ordered("PreToolUse", root, `{"notebook_path":`+quoted+`,"new_source":"`+big+`"}`), `"tool_name":"Write"`, `"tool_name":"NotebookEdit"`, 1)
	if got, err := decode(notebook); err != nil || got.Denial != "NotebookEdit is not supported by this adapter" {
		t.Fatalf("NotebookEdit over the bound = %+v, %v", got, err)
	}
	got, err := decode(ordered("PreToolUse", root, `{"file_path":`+quoted+`,"content":"`+big+`"}`))
	if err != nil || len(got.Event.Candidates) != 1 || got.Event.Candidates[0].Path != "big.md" || got.Event.Candidates[0].Op != model.CandidateReplace ||
		len(got.Gaps) != 1 || got.Gaps[0] != hook.OverInputBound("big.md") || got.Content != nil {
		t.Fatalf("a write over the bound = %+v, %v", got, err)
	}
	post := strings.Replace(ordered("PostToolUse", root, `{"file_path":`+quoted+`,"content":"`+big+`"}`), `"tool_use_id"`, `"tool_response":{"success":true},"tool_use_id"`, 1)
	if got, err = decode(post); err != nil || len(got.Hints) != 1 || got.Hints[0] != "big.md" || got.Event.Shape != model.ShapePost {
		t.Fatalf("a post event over the bound = %+v, %v", got, err)
	}
}
