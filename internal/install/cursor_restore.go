package install

import (
	"fmt"
	"slices"
)

// cursorMovedNote is the line a person reads for each golaw handler found
// away from the position its receipt records, as section 5.6 asks of every
// merge: the handler removed is golaw's own, found by its exact JSON, and
// whatever now stands at the recorded position is another tool's and stays.
const cursorMovedNote = "golaw: o hook do golaw em %s estava na posição %d e está na posição %d de %s."

// cursorRestoreHooks plans the removal of exactly the handlers the receipt
// records, each found by its exact JSON in its event wherever it stands,
// so another tool's handler at a recorded index stays and one the person
// already removed needs nothing. A hooks file the install created goes
// once nothing of anyone else is left in it.
func cursorRestoreHooks(plan *Plan) error {
	path := plan.Receipt.HooksPath
	if err := cursorUnlinked(path, cursorHooksDepth); err != nil {
		return err
	}
	data, exists, err := readOptional(path)
	if err != nil || !exists || len(plan.Receipt.Entries) == 0 {
		return err
	}
	file, err := parseCursorHooks(data)
	if err != nil {
		return fmt.Errorf("read the cursor hooks %s: %w", path, err)
	}
	owned := map[string][]int{}
	for _, entry := range plan.Receipt.Entries {
		index, located, ownErr := cursorOwnedEntry(&file, entry)
		if ownErr != nil {
			return fmt.Errorf("cursor hooks %s, receipt %s: %w", path, plan.ReceiptPath, ownErr)
		}
		if !located {
			continue
		}
		owned[entry.Event] = append(owned[entry.Event], index)
		if index != entry.Group {
			plan.Notes = append(plan.Notes, fmt.Sprintf(cursorMovedNote, entry.Event, entry.Group, index, path))
		}
	}
	file.remove(owned)
	if plan.Receipt.HooksCreated && file.empty() {
		return addChange(plan, path, nil, true)
	}
	remaining, err := file.encode()
	if err != nil {
		return err
	}
	return addChange(plan, path, remaining, false)
}

// cursorOwnedEntry locates the handler an entry records, after checking
// that the entry is one a Cursor installation writes: an event golaw
// installs, a handler of its own and a command HookCommand wrote, so a
// receipt never becomes the authority to remove another tool's handler.
func cursorOwnedEntry(file *cursorHooksFile, entry Entry) (int, bool, error) {
	if _, ok := cursorBinary(entry.Definition); !ok || entry.Handler != 0 ||
		!slices.Contains(cursorEvents(), entry.Event) {
		return 0, false, fmt.Errorf("receipt entry of %s is no handler a cursor installation writes: %w", entry.Event, ErrPath)
	}
	return file.owned(entry.Event, []Entry{entry})
}

// remove takes out the handlers at the indexes of each event, the last
// first, so a removal never shifts an index still to remove; an event the
// removal empties goes with its last handler, while an event another tool
// left empty stays.
func (f *cursorHooksFile) remove(owned map[string][]int) {
	for event, indexes := range owned {
		ordered := slices.Compact(slices.Sorted(slices.Values(indexes)))
		slices.Reverse(ordered)
		for _, index := range ordered {
			f.handlers[event] = slices.Delete(f.handlers[event], index, index+1)
		}
		if len(f.handlers[event]) == 0 {
			delete(f.handlers, event)
			f.events = slices.DeleteFunc(f.events, func(name string) bool { return name == event })
		}
	}
}

// empty reports a file with no event and no top-level field but the
// version and hooks: nothing in it belongs to anyone else.
func (f *cursorHooksFile) empty() bool {
	return len(f.events) == 0 && !slices.ContainsFunc(f.keys, func(key string) bool {
		return key != cursorVersionKey && key != cursorHooksKey
	})
}
