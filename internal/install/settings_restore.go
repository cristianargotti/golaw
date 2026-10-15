package install

import (
	"encoding/json"
	"slices"
)

// RestoreSettings removes only values still attributable to this installation.
func RestoreSettings(data []byte, owned Ownership) ([]byte, error) {
	root, err := settingsObject(data)
	if err != nil {
		return nil, err
	}
	err = restorePointers(root, owned)
	if err != nil {
		return nil, err
	}
	deny, _, err := denyValues(root)
	if err != nil {
		return nil, err
	}
	deny = removeOwned(deny, owned.Deny)
	if err := putDeny(root, deny, owned.HadDeny || len(deny) > 0); err != nil {
		return nil, err
	}
	if err := pruneContainers(root, owned.Containers); err != nil {
		return nil, err
	}
	if owned.Created && len(root) == 0 {
		return nil, nil
	}
	return encodeSettings(root)
}

func restorePointers(root map[string]json.RawMessage, owned Ownership) error {
	for _, key := range pointerKeys() {
		current, err := field(root, key)
		if err != nil {
			return err
		}
		if !equalJSON(current, owned.Installed[key]) {
			continue
		}
		original := owned.Original[key]
		if slices.Contains(owned.Missing, key) {
			original = nil
		}
		if err := putField(root, key, original); err != nil {
			return err
		}
	}
	return nil
}

func pruneContainers(root map[string]json.RawMessage, names []string) error {
	for _, name := range names {
		object, err := settingsObject(root[name])
		if err != nil {
			return err
		}
		if len(object) == 0 {
			delete(root, name)
		}
	}
	return nil
}
