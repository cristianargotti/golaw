package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// The Cursor events by the answer they take (D5): a permission with an
// agent message, a permission without one, and the additional context of a
// post event.
const (
	cursorPre     = "pre"
	cursorRead    = "read"
	cursorPost    = "post"
	keyPermission = "permission"
)

// cursorKind is the answer family of a Cursor event.
func cursorKind(event string) string {
	switch event {
	case cursorPostTool, cursorPostFailure:
		return cursorPost
	case cursorReadFile:
		return cursorRead
	default:
		return cursorPre
	}
}

// judgeCursor holds a reply of `golaw hook cursor` to D5: exit 0, nothing
// on stderr and one JSON object always, because under failClosed no output
// blocks, and exactly the fields of its event, because a field outside a
// hook's schema blocks too (survey section 8.2).
func judgeCursor(event string, want m4Want, got result) (m4Answer, error) {
	if got.err != nil {
		return m4Answer{}, fmt.Errorf("run the installed command: %w", got.err)
	}
	if got.code != 0 || got.stderr != "" {
		return m4Answer{}, fmt.Errorf("exit %d with stderr %q, want exit 0 and a JSON document: %w", got.code, got.stderr, ErrAcceptance)
	}
	document := map[string]string{}
	decoder := json.NewDecoder(strings.NewReader(got.stdout))
	if err := decoder.Decode(&document); err != nil {
		return m4Answer{}, fmt.Errorf("decode the answer %q: %w", got.stdout, err)
	}
	if decoder.More() || !strings.HasSuffix(got.stdout, "}\n") {
		return m4Answer{}, fmt.Errorf("stdout %q is not one JSON object and a line feed: %w", got.stdout, ErrAcceptance)
	}
	return cursorAnswer(cursorKind(event), want, document)
}

// cursorAnswer holds the document to the fields of its answer.
func cursorAnswer(kind string, want m4Want, document map[string]string) (m4Answer, error) {
	answer := m4Answer{Reason: document["agent_message"], Human: document["user_message"]}
	switch {
	case kind != cursorPost && want == m4Allow:
		return answer, cursorFields(document, wantAllow, keyPermission)
	case kind == cursorPre && want == m4Deny:
		err := cursorFields(document, wantDeny, "agent_message", keyPermission, "user_message")
		if err == nil && answer.Human == answer.Reason {
			err = fmt.Errorf("the human line repeats the English reason: %w", ErrAcceptance)
		}
		return answer, err
	case kind == cursorRead && want == m4Deny:
		return answer, cursorFields(document, wantDeny, keyPermission, "user_message")
	case kind == cursorPost && want == m4Allow:
		return answer, cursorFields(document, "")
	case kind == cursorPost && want == m4Block:
		return m4Answer{Reason: document["additional_context"]}, cursorFields(document, "", "additional_context")
	}
	return m4Answer{}, fmt.Errorf("no Cursor answer %d exists for a %s event: %w", want, kind, ErrAcceptance)
}

// cursorFields requires exactly the keys, every value non-empty, and the
// permission when one is named.
func cursorFields(document map[string]string, permission string, keys ...string) error {
	got := slices.Sorted(maps.Keys(document))
	if !slices.Equal(got, keys) {
		return fmt.Errorf("the answer holds %q, want %q: %w", got, keys, ErrAcceptance)
	}
	for _, key := range keys {
		if document[key] == "" {
			return fmt.Errorf("the answer field %s is empty: %w", key, ErrAcceptance)
		}
	}
	if permission != "" && document[keyPermission] != permission {
		return fmt.Errorf("the answer permission is %q, want %q: %w", document[keyPermission], permission, ErrAcceptance)
	}
	return nil
}
