package main

import "strings"

// The sizes of the fixture groups, for their allocation.
const (
	editCases      = 4
	policyCases    = 6
	protocolCases  = 6
	postWriteCases = 6
	postShellCases = 4
)

const (
	fileLimit      = 300
	overFileLimit  = 301
	functionLimit  = 40
	overFunction   = 41
	payloadLimit   = 8 << 20
	modeRaw        = "raw"
	modeShellPost  = "shell-post"
	modeUnenrolled = "unenrolled"
	modeUnreadable = "unreadable"
	modeDeleted    = "deleted-policy"
	modeWeakening  = "staged-weakening"
	modeUnstaged   = "staged-deletion"
	modeCommitted  = "committed-policy"
	pathNote       = "note.txt"
	pathSize       = "size.go"
	keyFilePath    = "file_path"
	textSafe       = "safe\n"
	textDash       = "one\u2014two\n"
	categoryPath   = "path"
	caseAlias      = "case-alias"
)

type fixture struct {
	name     string
	category string
	event    string
	tool     string
	want     string
	mode     string
	raw      string
	input    map[string]any
	before   map[string]string
	after    map[string]string
}

func writeFixture(name, path, content, want string) fixture {
	return fixture{
		name: name, category: "write", event: eventPre, tool: toolWrite, want: want,
		input: map[string]any{keyFilePath: path, "content": content},
	}
}

func fixtures() []fixture {
	out := writeFixtures()
	for _, group := range [][]fixture{
		readFixtures(), editFixtures(), shellFixtures(),
		protocolFixtures(), postWriteFixtures(), postShellFixtures(), policyFixtures(),
	} {
		out = append(out, group...)
	}
	return out
}

func writeFixtures() []fixture {
	return []fixture{
		writeFixture("dash-write", pathNote, textDash, wantDeny),
		writeFixture("clean-write", pathNote, "one-two\n", wantAllow),
		writeFixture("forbidden-path", "image.png", textSafe, wantDeny),
		writeFixture("allowed-path", "public/image.png", textSafe, wantAllow),
		writeFixture("write-300", pathSize, goLines(fileLimit), wantAllow),
		writeFixture("write-301", pathSize, goLines(overFileLimit), wantDeny),
		writeFixture("function-40", "function.go", functionLines(functionLimit), wantAllow),
		writeFixture("function-41", "function.go", functionLines(overFunction), wantDeny),
		writeFixture("test-function-41", "function_test.go", functionLines(overFunction), wantAllow),
	}
}

func readFixtures() []fixture {
	return []fixture{
		{
			name: "secret-read", category: "read", event: eventPre,
			tool: toolRead, want: wantDeny, input: map[string]any{keyFilePath: ".env"},
		},
		{
			name: "source-read", category: "read", event: eventPre,
			tool: toolRead, want: wantAllow, input: map[string]any{keyFilePath: "credentials.go"},
			before: map[string]string{"credentials.go": "package fixture\n"},
		},
		{
			name: "notebook", category: "protocol", event: eventPre,
			tool: toolNotebook, want: wantDeny,
			input: map[string]any{"notebook_path": "book.ipynb", "new_source": "1"},
		},
	}
}

func editFixtures() []fixture {
	out := make([]fixture, 0, editCases)
	for _, item := range []struct {
		name, path, before, after, want string
	}{
		{"dash-edit", pathNote, textSafe, textDash, wantDeny},
		{"clean-edit", pathNote, textSafe, "one-two\n", wantAllow},
		{"edit-300", pathSize, goLines(fileLimit), goLines(fileLimit), wantAllow},
		{"edit-301", pathSize, goLines(fileLimit), goLines(overFileLimit), wantDeny},
	} {
		out = append(out, fixture{
			name: item.name, category: "edit", event: eventPre,
			tool: toolEdit, want: item.want,
			before: map[string]string{item.path: item.before},
			input: map[string]any{
				keyFilePath: item.path, "old_string": item.before,
				"new_string": item.after, "replace_all": false,
			},
		})
	}
	return out
}

func policyFixtures() []fixture {
	out := make([]fixture, 0, policyCases)
	for _, mode := range []string{modeUnreadable, modeDeleted, modeWeakening, modeUnstaged, modeCommitted, modeUnenrolled} {
		f := writeFixture(mode, pathNote, textSafe, wantDeny)
		f.category, f.mode = "policy", mode
		switch mode {
		case modeUnenrolled, modeCommitted:
			f.want = wantAllow
		case modeUnreadable:
			f.category, f.tool, f.want = wantFailure, toolEdit, wantError
			f.input = map[string]any{keyFilePath: "target", "old_string": "a", "new_string": "b"}
		}
		out = append(out, f)
	}
	return out
}

func protocolFixtures() []fixture {
	out := make([]fixture, 0, protocolCases)
	for _, item := range []struct{ name, raw string }{
		{"malformed", "not-json"},
		{"empty", ""},
		{"truncated", `{"tool_name":"Write"`},
		{"oversized", strings.Repeat(" ", payloadLimit+1) + "{}"},
		{"foreign-envelope", `{"hook_event_name":"beforeShellExecution","command":"git status","workspace_roots":["."]}`},
		{"second-object", "{}{}"},
	} {
		out = append(out, fixture{
			name: item.name, category: "protocol", event: eventPre,
			tool: toolWrite, want: wantError, mode: modeRaw, raw: item.raw,
		})
	}
	return out
}

func postWriteFixtures() []fixture {
	out := make([]fixture, 0, postWriteCases)
	for _, tool := range []string{toolWrite, toolEdit, toolNotebook} {
		for _, lines := range []int{fileLimit, overFileLimit} {
			name, want := "post-"+tool, wantPost
			if lines == fileLimit {
				name, want = "post-control-"+tool, wantPostAllow
			}
			out = append(out, fixture{
				name: name, category: wantPost, event: eventPost, tool: tool, want: want,
				input:  map[string]any{keyFilePath: pathSize, "notebook_path": pathSize},
				before: map[string]string{pathSize: goLines(lines)},
			})
		}
	}
	return out
}

func postShellFixtures() []fixture {
	out := make([]fixture, 0, postShellCases)
	for _, tool := range []string{toolBash, toolPowerShell} {
		for _, failed := range []bool{false, true} {
			event, want, prefix := eventPost, wantPost, "post-"
			if failed {
				event, want, prefix = eventFailure, wantFailure, "failure-"
			}
			out = append(out, fixture{
				name: prefix + tool, category: wantPost, event: event,
				tool: tool, want: want, mode: modeShellPost,
				input:  map[string]any{keyCommand: "printf changed > note.txt; false"},
				before: map[string]string{pathNote: textSafe},
				after:  map[string]string{pathNote: textDash},
			})
		}
	}
	return out
}

func goLines(lines int) string {
	return "package fixture\n" + strings.Repeat("\n", lines-1)
}

func functionLines(countedBodyLines int) string {
	return "package fixture\n\nfunc example() {\n" +
		strings.Repeat("\t_ = 1\n", countedBodyLines) + "}\n"
}
