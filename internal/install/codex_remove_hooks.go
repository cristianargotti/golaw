package install

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
)

// removeCodexHooks plans the removal of exactly the handlers the receipt
// records, each found by its exact JSON under its event and matcher, so a
// handler another tool placed at a recorded index stays and one the person
// already removed needs nothing. Every foreign
// identity the removal moves is reported, and a hooks file the install
// created goes once nothing of anyone else is left in it.
func removeCodexHooks(plan *Plan) error {
	path := plan.Receipt.HooksPath
	data, exists, err := readOptional(path)
	if err != nil || !exists || len(plan.Receipt.Entries) == 0 {
		return err
	}
	file, err := parseCodexHooks(data)
	if err != nil {
		return fmt.Errorf("codex hooks %s: %w", path, err)
	}
	owned := make([]codexPosition, 0, len(plan.Receipt.Entries))
	for _, entry := range plan.Receipt.Entries {
		position, located, locateErr := locateRecorded(&file, entry, path)
		if locateErr != nil {
			return locateErr
		}
		if located {
			owned = append(owned, position)
		}
	}
	before := file.foreign(owned)
	if removeErr := file.removeHandlers(owned); removeErr != nil {
		return removeErr
	}
	plan.Notes = append(plan.Notes, codexMovedNotes(path, path, before, file.foreign(nil))...)
	if plan.Receipt.HooksCreated && file.empty() {
		return addChange(plan, path, nil, true)
	}
	remaining, err := file.encode()
	if err != nil {
		return err
	}
	return addChange(plan, path, remaining, false)
}

// removeHandlers removes the handlers at positions, the last first, so a
// removal never shifts a position still to remove.
func (f *codexHooksFile) removeHandlers(positions []codexPosition) error {
	ordered := slices.Clone(positions)
	slices.SortFunc(ordered, func(a, b codexPosition) int {
		return cmp.Or(cmp.Compare(b.group, a.group), cmp.Compare(b.handler, a.handler))
	})
	for _, position := range ordered {
		if err := f.removeHandler(position); err != nil {
			return err
		}
	}
	return nil
}

// removeHandler removes one handler. Its group goes with it only when the
// group holds nothing else, no other handler and no field beside its
// matcher, and the event goes once its last group went.
func (f *codexHooksFile) removeHandler(position codexPosition) error {
	groups := f.groups[position.event]
	keys, values, err := rawObjectFields(groups[position.group])
	if err != nil {
		return err
	}
	var handlers []json.RawMessage
	if json.Unmarshal(values[codexHooksKey], &handlers) != nil || position.handler >= len(handlers) {
		return fmt.Errorf("handlers of %s group %d: %w", position.event, position.group, ErrSettings)
	}
	handlers = slices.Delete(handlers, position.handler, position.handler+1)
	if len(handlers) == 0 && !slices.ContainsFunc(keys, func(key string) bool {
		return key != codexMatcherKey && key != codexHooksKey
	}) {
		groups = slices.Delete(groups, position.group, position.group+1)
	} else {
		values[codexHooksKey] = rawArray(handlers)
		group, encodeErr := rawObject(keys, values)
		if encodeErr != nil {
			return encodeErr
		}
		groups[position.group] = group
	}
	f.groups[position.event] = groups
	if len(groups) == 0 {
		delete(f.groups, position.event)
		f.events = slices.DeleteFunc(f.events, func(event string) bool { return event == position.event })
	}
	return nil
}

// empty reports a file with no event and no top-level field but hooks:
// nothing in it belongs to anyone else.
func (f *codexHooksFile) empty() bool {
	return len(f.events) == 0 && !slices.ContainsFunc(f.keys, func(key string) bool { return key != codexHooksKey })
}
