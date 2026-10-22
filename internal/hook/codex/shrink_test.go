package codex_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/codex"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

var errBrokenInput = errors.New("broken input")

// escaped turns each percent sign of a row into a backslash, so the row
// spells the JSON escapes it holds as written.
func escaped(text string) string { return strings.ReplaceAll(text, "%", "\x5c") }

// padded is a document with a first member of n bytes the hook never
// reads, so every other byte of the document follows a long string.
func padded(doc string, n int) []byte {
	return []byte(`{"pad":"` + strings.Repeat("x", n) + `",` + doc[1:])
}

// plainEnvelope is a Bash envelope with one more member of the given raw
// value.
func plainEnvelope(extra string) string {
	return `{"session_id":"session","turn_id":"turn","cwd":"/project","hook_event_name":"PreToolUse",` +
		`"tool_name":"Bash","tool_input":{"command":"go test ./..."},"tool_use_id":"tool","extra":` + extra + `}`
}

// The bounded reader accepts exactly the documents the standard decoder
// accepts, whatever the values it discards hold; its reading does not
// depend on the size of the input, so the documents stay small.
func TestBoundedReaderAcceptsWhatTheDecoderAccepts(t *testing.T) {
	t.Parallel()
	for _, extra := range []string{
		`0`, `-0`, `12`, `-12.5e+10`, `1E5`, `0.0`, `1e-3`, `01`, `-`, `1.`, `.5`, `1e`, `1e+`, `--1`, `+1`, `1.5.2`, `2e3e4`, `-a`,
		`true`, `false`, `null`, `tru`, `nul`, `nulx`, `True`, `x`,
		escaped(`"a%nb"`), escaped(`"%u00e9"`), escaped(`"%ud83d%ude00"`), escaped(`"%ud800"`), escaped(`"%x"`), escaped(`"%u12g4"`),
		"\"a\tb\"", escaped(`"%/%b%f%r%t%%%""`), `"open`, "\"\xff\"",
		`[]`, `{}`, `[1,2,[3,{"a":null}]]`, `{"a":1,}`, `[1,]`, `[1 2]`, `{"a" 1}`, `{1:2}`, `{"a":1`, ` [ 1 , { "b" : [ ] } ] `,
		strings.Repeat("[", 9999) + strings.Repeat("]", 9999),
		strings.Repeat("[", 10000) + strings.Repeat("]", 10000),
		`1}trailing`, `1} `, "1}\n\t ", `1}{}`,
	} {
		doc := []byte(plainEnvelope(extra))
		if _, err := codex.Oversized(bytes.NewReader(doc)); (err == nil) != json.Valid(doc) {
			t.Errorf("value %.40q: bounded error %v, valid %v", extra, err, json.Valid(doc))
		}
	}
}

// parsedAs is what an event parses and decodes to, or the error that
// stopped it; the response of a post event is left out, because a
// response too long to keep stands as an empty one.
func parsedAs(t *testing.T, parsed hook.Parsed, err error, root string) any {
	t.Helper()
	if err != nil {
		return "parse: " + err.Error()
	}
	request, err := parsed.Decode(hook.Base{Root: root}, proof.NewTree(nil))
	if err != nil {
		return []any{parsed.Event, parsed.Targets, "decode: " + err.Error()}
	}
	if request.Event.Post != nil {
		request.Event.Post.Response = ""
	}
	return []any{parsed.Event, parsed.Targets, request}
}

// The bounded reader reads the envelope as the decoder reads it: names
// matched regardless of case, the last of a repeated member winning,
// every field type checked, a field too long to keep standing in where
// its presence is all the hook reads.
func TestBoundedReaderParsesAsTheDecoder(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("y", 70<<10)
	head := `{"session_id":"session","turn_id":"turn","cwd":"/project","hook_event_name":"PreToolUse","tool_name":"Bash",`
	for _, doc := range []string{
		plainEnvelope(`{"a":[1,{"b":"c"}]}`),
		`{"SESSION_ID":"session","Turn_Id":"turn","CWD":"/project","hook_event_name":"PreToolUse","TOOL_NAME":"Bash","tool_input":{"COMMAND":"git commit --no-verify -m x"},"tool_use_id":"tool"}`,
		head + `"tool_input":{"command":"go test"},"tool_input":{"command":"cat > notes.md <<'EOF'\nhello\nEOF"},"tool_use_id":"tool"}`,
		head + `"tool_input":{"command":"go test","command":"apply_patch <<'EOF'\n*** Begin Patch\n*** Add File: a.md\n+a\n*** End Patch\nEOF"},"tool_use_id":"tool"}`,
		head + `"tool_input":[1],"tool_use_id":"tool"}`,
		head + `"tool_input":"` + long + `","tool_use_id":"tool"}`,
		head + `"tool_input":{"command":1},"tool_use_id":"tool"}`,
		head + `"tool_input":{"command":null},"tool_use_id":"tool"}`,
		head + `"tool_input":{"command":{"a":["` + long + `"]}},"tool_use_id":"tool"}`,
		head + `"tool_input":{"command":["` + long + `"]},"tool_use_id":"tool"}`,
		head + `"tool_input":{"other":"go test"},"tool_use_id":"tool"}`,
		head + `"tool_use_id":"tool"}`,
		head + escaped(`"tool_input":{"command":"git commit -m %"x %u0041%" --no-verify"},"tool%u005fuse_id":"tool"}`),
		head + `"tool_input":{"command":"go test"},"tool_use_id":"tool","agent_id":"agent","conversation_id":null}`,
		head + `"tool_input":{"command":"go test"},"tool_use_id":"tool","workspace_roots":["` + long + `"]}`,
		head + `"tool_input":{"command":"go test"},"tool_use_id":"tool","` + strings.Repeat("k", 300) + `":1}`,
		`{"session_id":"session","turn_id":"turn","cwd":5,"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"go test"},"tool_use_id":"tool"}`,
		`{"session_id":"session","turn_id":"turn","cwd":"/project","hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"go test"},"tool_response":"` + long + `","tool_use_id":"tool"}`,
		`{"session_id":"session","turn_id":"turn","cwd":"/project","hook_event_name":"PostToolUse","tool_name":"apply_patch","tool_input":{"command":"*** Begin Patch\n*** Delete File: a.md\n*** End Patch"},"tool_response":{"output":[1,true]},"tool_use_id":"tool"}`,
	} {
		root := hooktest.Repository(t)
		data := hooktest.Payload([]byte(doc), root)
		within, withinErr := codex.Parse(data)
		bounded, boundedErr := codex.Oversized(bytes.NewReader(data))
		want, got := parsedAs(t, within, withinErr, root), parsedAs(t, bounded, boundedErr, root)
		if !reflect.DeepEqual(want, got) {
			t.Errorf("document %.100s\nwithin the bound %+v\nbounded %+v", doc, want, got)
		}
	}
}

// A field the hook reads that is longer than the bounded reader keeps
// cannot be judged, never read short, and a document that is no object is
// refused.
func TestBoundedReaderRefusesAFieldItCannotKeep(t *testing.T) {
	t.Parallel()
	doc := `{"session_id":"session","turn_id":"turn","cwd":"/` + strings.Repeat("d", 70<<10) + `","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"go test"},"tool_use_id":"tool"}`
	_, err := codex.Oversized(bytes.NewReader([]byte(doc)))
	if !errors.Is(err, model.ErrCannotJudge) || !strings.Contains(err.Error(), "cwd") {
		t.Fatalf("want the cwd named as cannot judge, got %v", err)
	}
	if _, err = codex.Oversized(bytes.NewReader([]byte(" [1]"))); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatalf("a document that is no object: %v", err)
	}
}

// A read that fails is cannot judge with its cause, inside a long string,
// between members, inside a number and a literal, and after the document.
func TestBoundedReaderReadFailures(t *testing.T) {
	t.Parallel()
	whole, literal := padded(plainEnvelope("1"), 200<<10), padded(plainEnvelope("true"), 200<<10)
	for _, input := range []io.Reader{
		io.MultiReader(bytes.NewReader(whole[:100<<10]), iotest.ErrReader(errBrokenInput)),
		io.MultiReader(bytes.NewReader(whole[:len(whole)-len(`"tool","extra":1}`)]), iotest.ErrReader(errBrokenInput)),
		io.MultiReader(bytes.NewReader(whole[:len(whole)-len(`1}`)]), iotest.ErrReader(errBrokenInput)),
		io.MultiReader(bytes.NewReader(whole[:len(whole)-len(`}`)]), iotest.ErrReader(errBrokenInput)),
		io.MultiReader(bytes.NewReader(whole), iotest.ErrReader(errBrokenInput)),
		io.MultiReader(bytes.NewReader(literal[:len(literal)-len(`ue}`)]), iotest.ErrReader(errBrokenInput)),
	} {
		if _, err := codex.Oversized(input); !errors.Is(err, model.ErrCannotJudge) || !errors.Is(err, errBrokenInput) {
			t.Fatalf("want cannot judge with its cause, got %v", err)
		}
	}
}

// The header paths of a patch over the bound are decoded as the standard
// decoder decodes them: escapes, surrogate pairs, halves left alone and
// bytes that are not valid UTF-8.
func TestBoundedReaderDecodesHeadersAsTheDecoder(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		escaped(`n%u00f3tas.md`), escaped(`%ud83d%ude00.md`), escaped(`%ud800x.md`), escaped(`%udc00.md`),
		escaped(`%ud800%ud800.md`), escaped(`a%/b.md`), escaped(`%ud800%u0041.md`), escaped(`%ud800%%b.md`),
		escaped(`%u00E9%u00e9.md`), "a\xffb.md", `plain.md`,
	} {
		doc := func(content string) string {
			return `{"session_id":"session","turn_id":"turn","cwd":"/project","hook_event_name":"PreToolUse","tool_name":"apply_patch",` +
				`"tool_input":{"command":"*** Begin Patch\n*** Add File: ` + path + `\n+` + content + `\n*** End Patch\n"},"tool_use_id":"tool"}`
		}
		root := hooktest.Repository(t)
		small := decodeRequest(t, []byte(doc("x")), nil)
		parsed, err := codex.Oversized(bytes.NewReader(hooktest.Payload([]byte(doc(strings.Repeat("x", hook.MaxInputBytes))), root)))
		if err != nil {
			t.Fatalf("%q: %v", path, err)
		}
		large, err := parsed.Decode(hook.Base{Root: root}, proof.NewTree(nil))
		if err != nil || len(large.Event.Candidates) != 1 || len(small.Event.Candidates) != 1 ||
			large.Event.Candidates[0].Path != small.Event.Candidates[0].Path {
			t.Fatalf("%q: within the bound %+v, over it %+v, %v", path, small.Event.Candidates, large.Event.Candidates, err)
		}
	}
}
