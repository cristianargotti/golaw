package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

func settingsObject(data []byte) (map[string]json.RawMessage, error) {
	root := map[string]json.RawMessage{}
	if len(data) == 0 {
		return root, nil
	}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("decode settings: %w", err)
	}
	if root == nil {
		return nil, fmt.Errorf("settings object: %w", ErrSettings)
	}
	return root, nil
}

func field(root map[string]json.RawMessage, key string) (json.RawMessage, error) {
	group, name, _ := strings.Cut(key, "/")
	object, err := settingsObject(root[group])
	if err != nil {
		return nil, err
	}
	return object[name], nil
}

func putField(root map[string]json.RawMessage, key string, value json.RawMessage) error {
	group, name, _ := strings.Cut(key, "/")
	object, err := settingsObject(root[group])
	if err != nil {
		return err
	}
	if value == nil {
		delete(object, name)
	} else {
		object[name] = value
	}
	encoded, err := json.Marshal(object)
	if err != nil {
		return fmt.Errorf("encode settings group: %w", err)
	}
	root[group] = encoded
	return nil
}

func denyValues(root map[string]json.RawMessage) ([]string, bool, error) {
	raw, err := field(root, denyKey)
	if err != nil || raw == nil {
		return nil, false, err
	}
	var deny []string
	if err := json.Unmarshal(raw, &deny); err != nil {
		return nil, false, fmt.Errorf("decode deny rules: %w", err)
	}
	return deny, true, nil
}

func putDeny(root map[string]json.RawMessage, deny []string, present bool) error {
	if !present {
		return putField(root, denyKey, nil)
	}
	if deny == nil {
		deny = []string{}
	}
	raw, err := json.Marshal(deny)
	if err != nil {
		return fmt.Errorf("encode deny rules: %w", err)
	}
	return putField(root, denyKey, raw)
}

func encodeSettings(root map[string]json.RawMessage) ([]byte, error) {
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode settings: %w", err)
	}
	return append(data, '\n'), nil
}

func equalJSON(left, right json.RawMessage) bool {
	if bytes.Equal(left, right) {
		return true
	}
	var a, b any
	if json.Unmarshal(left, &a) != nil || json.Unmarshal(right, &b) != nil {
		return false
	}
	return reflect.DeepEqual(a, b)
}
