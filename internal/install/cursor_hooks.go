package install

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/cristianargotti/golaw/internal/assets"
)

const (
	cursorHookWord   = "hook"
	cursorHooksAsset = "agents/cursor/hooks.json"
	cursorPreMatcher = "^(Write|Delete|Shell)$"
	cursorTimeout    = 10
)

// cursorEvents are the events golaw installs, in the order the receipt
// lists them; afterFileEdit has no answer and is not used.
func cursorEvents() []string {
	return []string{"preToolUse", "beforeReadFile", "beforeShellExecution", "postToolUse", "postToolUseFailure"}
}

// cursorAssetHandler is a handler of the embedded definition, in the
// field order the hooks file shows.
type cursorAssetHandler struct {
	Command    string `json:"command"`
	Matcher    string `json:"matcher,omitempty"`
	Timeout    int    `json:"timeout"`
	FailClosed bool   `json:"failClosed"`
}

type cursorAssetFile struct {
	Version int                             `json:"version"`
	Hooks   map[string][]cursorAssetHandler `json:"hooks"`
}

// cursorDefinition renders the embedded definition with the command of the
// running binary, so the hooks name the binary that installed them. The
// command is decoded again before it is handed out, because a hooks file
// that names another binary would run it on every tool call.
func cursorDefinition(options Options) (map[string][]json.RawMessage, error) {
	command := HookCommand(options.Executable, options.GOOS, cursorHookWord, cursorAgent)
	if binary, ok := CommandBinary(command); !ok || binary != options.Executable {
		return nil, fmt.Errorf("hook command %q does not name %s: %w", command, options.Executable, ErrPath)
	}
	asset, err := assets.Read(cursorHooksAsset)
	if err != nil {
		return nil, fmt.Errorf("load the cursor hooks: %w", err)
	}
	return renderCursorHandlers(asset, command)
}

// renderCursorHandlers refuses an asset that is not one blocking handler
// per installed event with the timeout the internal deadline stays below
// and the anchored preToolUse matcher, because a softer definition would
// let a tool call through unjudged without a word.
func renderCursorHandlers(asset []byte, command string) (map[string][]json.RawMessage, error) {
	var file cursorAssetFile
	if err := strictJSON(asset, &file); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrHooks, err)
	}
	if file.Version != 1 || len(file.Hooks) != len(cursorEvents()) {
		return nil, fmt.Errorf("%w: the cursor event set differs", ErrHooks)
	}
	wanted := make(map[string][]json.RawMessage, len(file.Hooks))
	for _, event := range cursorEvents() {
		handlers := file.Hooks[event]
		if len(handlers) != 1 || handlers[0].Timeout != cursorTimeout || !handlers[0].FailClosed ||
			(event == cursorEvents()[0] && handlers[0].Matcher != cursorPreMatcher) {
			return nil, fmt.Errorf("%w: cursor %s is not one blocking handler", ErrHooks, event)
		}
		handlers[0].Command = command
		definition, err := marshal(handlers[0])
		if err != nil {
			return nil, err
		}
		wanted[event] = []json.RawMessage{definition}
	}
	return wanted, nil
}

// cursorHooks merges golaw's handlers into the hooks file of the scope and
// records each one in the receipt at the index Cursor lists it by.
func cursorHooks(plan *Plan, options Options, previous Receipt) error {
	path := plan.Receipt.HooksPath
	if err := cursorUnlinked(path, cursorHooksDepth); err != nil {
		return err
	}
	if otherHooksFile(previous.HooksPath, path) {
		return fmt.Errorf("the cursor receipt records the hooks in %s, not %s: run golaw agent uninstall --agent cursor, which removes them there, then install again: %w",
			previous.HooksPath, path, ErrChanged)
	}
	wanted, err := cursorDefinition(options)
	if err != nil {
		return err
	}
	current, exists, err := readOptional(path)
	if err != nil {
		return err
	}
	merged, entries, err := cursorMerge(current, wanted, previous.Entries)
	if err != nil {
		return fmt.Errorf("cursor hooks %s, receipt %s: %w", path, plan.ReceiptPath, err)
	}
	plan.Receipt.Entries = entries
	plan.Receipt.HooksCreated = !exists || previous.HooksCreated
	return addChange(plan, path, merged, false)
}

// owned answers the index of the handler a previous entry of event records,
// wherever it stands today. A handler that appears twice, or one whose
// recorded index now holds another handler or that still names golaw's
// binary in another form, is refused, naming what to fix, because
// replacing or removing it would act on what the person wrote; one the
// person removed is not located, and no error.
func (f *cursorHooksFile) owned(event string, previous []Entry) (int, bool, error) {
	for _, entry := range previous {
		if entry.Event != event {
			continue
		}
		found := f.locate(event, entry.Definition)
		switch {
		case len(found) == 1:
			return found[0], true, nil
		case len(found) > 1:
			return 0, false, fmt.Errorf("the golaw %s hook appears %d times: remove the copies so one is left: %w", event, len(found), ErrChanged)
		case entry.Group < len(f.handlers[event]) || f.namesBinaryOf(entry):
			return 0, false, fmt.Errorf("the golaw %s hook at position %d differs from the one its receipt records: put back the definition the receipt records and run the command again: %w",
				event, entry.Group, ErrChanged)
		}
		return 0, false, nil
	}
	return 0, false, nil
}

// locate compares handlers as canonical JSON, so a file an editor rewrote
// with its keys in another order still shows golaw's handler, and answers
// every index, so a copy is never missed.
func (f *cursorHooksFile) locate(event string, definition json.RawMessage) []int {
	want, ok := canonicalJSON(definition)
	if !ok {
		return nil
	}
	var found []int
	for index, candidate := range f.handlers[event] {
		if got, ok := canonicalJSON(candidate); ok && bytes.Equal(got, want) {
			found = append(found, index)
		}
	}
	return found
}

// namesBinaryOf tells an edited handler from a removed one: an edit keeps
// naming golaw's binary, which only golaw's handler does.
func (f *cursorHooksFile) namesBinaryOf(entry Entry) bool {
	recorded, ok := cursorBinary(entry.Definition)
	if !ok {
		return false
	}
	for _, handler := range f.handlers[entry.Event] {
		if binary, ok := cursorBinary(handler); ok && binary == recorded {
			return true
		}
	}
	return false
}

// cursorBinary reads the binary back only from a command HookCommand
// wrote, so a command of another tool never counts as golaw's.
func cursorBinary(raw json.RawMessage) (string, bool) {
	var fields map[string]json.RawMessage
	var command string
	if json.Unmarshal(raw, &fields) != nil || json.Unmarshal(fields[cursorCommandKey], &command) != nil {
		return "", false
	}
	return CommandBinary(command)
}

// cursorMatcherOf is the matcher a handler carries, empty for the events
// Cursor runs for every call.
func cursorMatcherOf(raw json.RawMessage) string {
	var handler cursorAssetHandler
	if json.Unmarshal(raw, &handler) != nil {
		return ""
	}
	return handler.Matcher
}
