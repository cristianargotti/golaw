package install

import (
	"encoding/json"
	"fmt"
	"maps"
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
	return encodeSettings(root, data)
}

// VerifySettings judges a settings file by the entries this installation
// owns, because every other entry belongs to the person and may change.
func VerifySettings(data []byte, owned Ownership) error {
	root, err := settingsObject(data)
	if err != nil {
		return err
	}
	for _, key := range slices.Sorted(maps.Keys(owned.Installed)) {
		current, fieldErr := field(root, key)
		if fieldErr != nil {
			return fieldErr
		}
		if !equalJSON(current, owned.Installed[key]) {
			return fmt.Errorf("owned setting %s: %w", key, ErrChanged)
		}
	}
	deny, _, err := denyValues(root)
	if err != nil {
		return err
	}
	for _, rule := range owned.Deny {
		if !slices.Contains(deny, rule) {
			return fmt.Errorf("owned deny rule %s: %w", rule, ErrChanged)
		}
	}
	return nil
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
