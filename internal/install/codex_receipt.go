package install

import (
	"encoding/json"
	"fmt"
	"os"
)

// codexMovedNote is the line a person reads when a trust identity moved:
// Codex keys the approval by position, so the handler under the new key
// waits for a new review in /hooks.
const codexMovedNote = "golaw: a identidade de confiança %s mudou para %s; revise-a no Codex com /hooks."

// finishCodexReceipt schedules the receipt after every other change, so
// a run that stops early never leaves a receipt naming files it did not
// write.
func finishCodexReceipt(plan *Plan) error {
	encoded, err := json.MarshalIndent(plan.Receipt, "", "  ")
	if err != nil {
		return fmt.Errorf("encode the codex receipt: %w", err)
	}
	return addChange(plan, plan.ReceiptPath, append(encoded, '\n'), false)
}

func previousEntry(receipt Receipt, event string) (Entry, bool) {
	for _, entry := range receipt.Entries {
		if entry.Event == event {
			return entry, true
		}
	}
	return Entry{}, false
}

func positionsOf(entries []Entry) []codexPosition {
	positions := make([]codexPosition, 0, len(entries))
	for _, entry := range entries {
		positions = append(positions, codexPosition{event: entry.Event, group: entry.Group, handler: entry.Handler})
	}
	return positions
}

// codexMovedNotes names every handler whose trust key a change moved,
// pairing the positions before and after in order, because Codex keeps
// the approval under the old key and runs nothing under the new one until
// the person reviews it; the key carries the hooks file too, so a file
// reached under another name counts as a move.
func codexMovedNotes(oldPath, newPath string, before, after []codexPosition) []string {
	var notes []string
	for index := range min(len(before), len(after)) {
		old, moved := before[index], after[index]
		oldKey := CodexTrustKey(oldPath, old.event, old.group, old.handler)
		newKey := CodexTrustKey(newPath, moved.event, moved.group, moved.handler)
		if oldKey != newKey {
			notes = append(notes, fmt.Sprintf(codexMovedNote, oldKey, newKey))
		}
	}
	return notes
}

// locateRecorded finds the one handler an entry records, wherever it
// stands today. When no handler under its event names the binary the
// entry names any more, the person removed it and nothing of it is left:
// it is not located, and no error. A handler that still names the binary
// but differs, or one that appears twice, is refused, naming what to fix,
// because replacing or removing it would act on what the person wrote.
func locateRecorded(file *codexHooksFile, entry Entry, path string) (codexPosition, bool, error) {
	found := file.locate(entry.Event, entry.Matcher, entry.Definition)
	switch {
	case len(found) == 1:
		return found[0], true, nil
	case len(found) > 1:
		return codexPosition{}, false, fmt.Errorf("the golaw %s handler appears %d times in %s: remove the copies so one is left: %w",
			entry.Event, len(found), path, ErrChanged)
	case file.namesBinaryOf(entry):
		return codexPosition{}, false, fmt.Errorf("the golaw %s handler in %s differs from the one its receipt records: restore it as recorded, or remove it and its receipt: %w",
			entry.Event, path, ErrChanged)
	}
	return codexPosition{}, false, nil
}

// namesBinaryOf tells an edited handler from a removed one: an edit keeps
// naming golaw's binary, which only golaw's handler does.
func (f *codexHooksFile) namesBinaryOf(entry Entry) bool {
	recorded := codexBinaries(entry.Definition)
	for _, raw := range f.groups[entry.Event] {
		group, err := decodeCodexGroup(raw)
		if err != nil {
			continue
		}
		for _, handler := range group.hooks {
			for binary := range codexBinaries(handler) {
				if recorded[binary] {
					return true
				}
			}
		}
	}
	return false
}

// codexBinaries reads the binaries back only from commands HookCommand
// wrote, so a command of another tool never counts as golaw's.
func codexBinaries(raw json.RawMessage) map[string]bool {
	fields, err := decodeCodexHandler(raw)
	if err != nil {
		return nil
	}
	binaries := map[string]bool{}
	for _, command := range []*string{fields.command, fields.windows} {
		if command == nil {
			continue
		}
		if binary, ok := CommandBinary(*command); ok {
			binaries[binary] = true
		}
	}
	return binaries
}

// otherHooksFile reports a receipt whose hooks file is not the one an
// installation is about to write and still exists. A name that reaches the
// same file through a link is the same file.
func otherHooksFile(recorded, current string) bool {
	if recorded == current {
		return false
	}
	recordedInfo, err := os.Stat(recorded)
	if err != nil {
		return false
	}
	currentInfo, err := os.Stat(current)
	return err != nil || !os.SameFile(recordedInfo, currentInfo)
}
