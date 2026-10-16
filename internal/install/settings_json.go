package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
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
	encoded, err := encodeObject(object, keyOrder(root[group]))
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
	raw, err := marshal(deny)
	if err != nil {
		return fmt.Errorf("encode deny rules: %w", err)
	}
	return putField(root, denyKey, raw)
}

// encodeSettings keeps the keys in the order of the person's file and only
// re-indents the document, so a merge changes the layout, never a value.
func encodeSettings(root map[string]json.RawMessage, original []byte) ([]byte, error) {
	compact, err := encodeObject(root, keyOrder(original))
	if err != nil {
		return nil, fmt.Errorf("encode settings: %w", err)
	}
	var out bytes.Buffer
	err = json.Indent(&out, compact, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("indent settings: %w", err)
	}
	return append(out.Bytes(), '\n'), nil
}

// keyOrder lists the keys of a JSON object in the order they are written,
// because a Go map forgets it; anything but an object has no order.
func keyOrder(data []byte) []string {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return nil
	}
	var keys []string
	for decoder.More() {
		token, err := decoder.Token()
		key, isKey := token.(string)
		var value json.RawMessage
		if err != nil || !isKey || decoder.Decode(&value) != nil {
			return keys
		}
		if !slices.Contains(keys, key) {
			keys = append(keys, key)
		}
	}
	return keys
}

// encodeObject writes the known keys in their order and the new ones after
// them, sorted, so an added container lands last and the output is stable.
func encodeObject(values map[string]json.RawMessage, order []string) ([]byte, error) {
	keys := slices.DeleteFunc(slices.Clone(order), func(key string) bool {
		_, kept := values[key]
		return !kept
	})
	for _, key := range slices.Sorted(maps.Keys(values)) {
		if !slices.Contains(keys, key) {
			keys = append(keys, key)
		}
	}
	out := []byte{'{'}
	for index, key := range keys {
		if index > 0 {
			out = append(out, ',')
		}
		name, err := marshal(key)
		if err != nil {
			return nil, err
		}
		out = append(append(append(out, name...), ':'), values[key]...)
	}
	return append(out, '}'), nil
}

// marshal encodes without HTML escaping, because the settings file is read
// by the person who wrote it: an ampersand of their hook command turned into
// \u0026 is a change they did not make.
func marshal(value any) ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, fmt.Errorf("encode settings value: %w", err)
	}
	return bytes.TrimSuffix(out.Bytes(), []byte("\n")), nil
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
