package install

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/cristianargotti/golaw/internal/assets"
)

const (
	codexPreToolUse  = "PreToolUse"
	codexPostToolUse = "PostToolUse"
	codexHooksAsset  = "agents/codex/hooks.json"
	codexMatcher     = "^(Bash|apply_patch)$"
	codexTimeout     = 10
)

// codexEvents are the two events golaw installs, in the order the receipt
// lists them; Codex has no failure event, and a failed command still
// reaches PostToolUse.
func codexEvents() []string {
	return []string{codexPreToolUse, codexPostToolUse}
}

type codexOwned struct {
	event, matcher string
	definition     json.RawMessage
}

// codexAssetHandler is the handler of the embedded definition, in the
// field order the hooks file shows; commandWindows is written for a
// Windows target only, and no failure field exists for Codex to read.
type codexAssetHandler struct {
	Type           string `json:"type"`
	Command        string `json:"command"`
	CommandWindows string `json:"commandWindows,omitempty"`
	Timeout        int    `json:"timeout"`
}

type codexAssetFile struct {
	Hooks map[string][]struct {
		Matcher string              `json:"matcher"`
		Hooks   []codexAssetHandler `json:"hooks"`
	} `json:"hooks"`
}

// codexHandlers renders the embedded definition with the commands of the
// running binary, so the hooks name the binary that installed them.
func codexHandlers(options Options) ([]codexOwned, error) {
	command, windows, err := codexCommands(options)
	if err != nil {
		return nil, err
	}
	asset, err := assets.Read(codexHooksAsset)
	if err != nil {
		return nil, fmt.Errorf("load the codex hooks: %w", err)
	}
	return renderCodexHandlers(asset, command, windows)
}

// renderCodexHandlers refuses an asset that is not exactly one command
// group per installed event under the anchored matcher with the timeout
// the internal deadline stays below, because a softer definition would
// leave tool calls unjudged without a word.
func renderCodexHandlers(asset []byte, command, windows string) ([]codexOwned, error) {
	var file codexAssetFile
	if err := strictJSON(asset, &file); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrHooks, err)
	}
	if len(file.Hooks) != len(codexEvents()) {
		return nil, fmt.Errorf("%w: the codex event set differs", ErrHooks)
	}
	owned := make([]codexOwned, 0, len(file.Hooks))
	for _, event := range codexEvents() {
		groups := file.Hooks[event]
		if len(groups) != 1 || len(groups[0].Hooks) != 1 || groups[0].Matcher != codexMatcher {
			return nil, fmt.Errorf("%w: codex %s is not one group under %s", ErrHooks, event, codexMatcher)
		}
		handler := groups[0].Hooks[0]
		if handler.Type != codexCommand || handler.Timeout != codexTimeout || handler.CommandWindows != "" {
			return nil, fmt.Errorf("%w: codex %s handler differs", ErrHooks, event)
		}
		handler.Command, handler.CommandWindows = command, windows
		definition, err := json.Marshal(handler)
		if err != nil {
			return nil, fmt.Errorf("encode the codex handler: %w", err)
		}
		owned = append(owned, codexOwned{event: event, matcher: codexMatcher, definition: definition})
	}
	return owned, nil
}

// addCodexHooks merges golaw's handlers into the hooks file: a handler the
// previous receipt records is found by its exact JSON under its event and
// matcher and replaced where it stands, a new one is appended after every
// group of its event, and every trust identity that moved since the
// receipt is reported, because Codex no longer applies the approval the
// person gave under the old key.
func addCodexHooks(plan *Plan, previous Receipt, options Options) error {
	owned, err := codexHandlers(options)
	if err != nil {
		return err
	}
	path := plan.Receipt.HooksPath
	data, exists, err := readOptional(path)
	if err != nil {
		return err
	}
	file, err := parseCodexHooks(data)
	if err != nil {
		return fmt.Errorf("codex hooks %s: %w", path, err)
	}
	recorded, err := recordedCodexEntries(&file, previous, path, exists)
	if err != nil {
		return err
	}
	before := file.foreign(positionsOf(slices.Collect(maps.Values(recorded))))
	plan.Receipt.HooksCreated = !exists || previous.HooksCreated
	for _, handler := range owned {
		entry, placeErr := placeCodexHandler(&file, handler, recorded)
		if placeErr != nil {
			return placeErr
		}
		plan.Receipt.Entries = append(plan.Receipt.Entries, entry)
		if _, located := recorded[handler.event]; located {
			old, _ := previousEntry(previous, handler.event)
			plan.Notes = append(plan.Notes, codexMovedNotes(previous.HooksPath, path,
				positionsOf([]Entry{old}), positionsOf([]Entry{entry}))...)
		}
	}
	plan.Notes = append(plan.Notes, codexMovedNotes(path, path, before, file.foreign(positionsOf(plan.Receipt.Entries)))...)
	merged, err := file.encode()
	if err != nil {
		return err
	}
	return addChange(plan, path, merged, false)
}

// recordedCodexEntries answers, by event, each entry of the previous
// receipt at the position its handler holds in the file today; an entry
// whose handler the person removed is left out, so it is written again. A
// receipt whose hooks file is another file that still exists is refused:
// its handlers there would keep running with no receipt naming them.
func recordedCodexEntries(file *codexHooksFile, previous Receipt, path string, exists bool) (map[string]Entry, error) {
	recorded := map[string]Entry{}
	if len(previous.Entries) == 0 {
		return recorded, nil
	}
	if otherHooksFile(previous.HooksPath, path) {
		return nil, fmt.Errorf("the codex receipt records the hooks in %s, not %s: run golaw agent uninstall --agent codex, which removes them there, then install again: %w",
			previous.HooksPath, path, ErrChanged)
	}
	if !exists {
		return recorded, nil
	}
	for _, entry := range previous.Entries {
		position, located, err := locateRecorded(file, entry, path)
		if err != nil {
			return nil, err
		}
		if located {
			entry.Group, entry.Handler = position.group, position.handler
			recorded[entry.Event] = entry
		}
	}
	return recorded, nil
}

// placeCodexHandler replaces golaw's handler where it stands, so its
// trust key stays, or appends a group of its own, so no other handler
// moves.
func placeCodexHandler(file *codexHooksFile, handler codexOwned, recorded map[string]Entry) (Entry, error) {
	entry := Entry{Event: handler.event, Matcher: handler.matcher, Definition: handler.definition}
	if current, found := recorded[handler.event]; found {
		position := codexPosition{event: handler.event, group: current.Group, handler: current.Handler}
		if err := file.replaceHandler(position, handler.definition); err != nil {
			return Entry{}, err
		}
		entry.Group, entry.Handler = current.Group, current.Handler
		return entry, nil
	}
	group, err := json.Marshal(struct {
		Matcher string            `json:"matcher"`
		Hooks   []json.RawMessage `json:"hooks"`
	}{handler.matcher, []json.RawMessage{handler.definition}})
	if err != nil {
		return Entry{}, fmt.Errorf("encode the codex group: %w", err)
	}
	position := file.appendGroup(handler.event, group)
	entry.Group, entry.Handler = position.group, position.handler
	return entry, nil
}
