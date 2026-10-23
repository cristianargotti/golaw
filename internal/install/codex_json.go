package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// rawObjectFields reads one JSON object into its keys in order and their
// raw values; a repeated key is refused, since which one a reader keeps
// is not certain.
func rawObjectFields(data []byte) ([]string, map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return nil, nil, fmt.Errorf("no JSON object: %w", ErrSettings)
	}
	var keys []string
	values := map[string]json.RawMessage{}
	for decoder.More() {
		token, err := decoder.Token()
		key, isKey := token.(string)
		var value json.RawMessage
		if err != nil || !isKey || decoder.Decode(&value) != nil {
			return nil, nil, fmt.Errorf("malformed JSON object: %w", ErrSettings)
		}
		if _, repeated := values[key]; repeated {
			return nil, nil, fmt.Errorf("key %q appears twice: %w", key, ErrSettings)
		}
		keys, values[key] = append(keys, key), value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, nil, fmt.Errorf("unterminated JSON object: %w", ErrSettings)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, nil, fmt.Errorf("trailing JSON: %w", ErrSettings)
	}
	return keys, values, nil
}

// rawObject writes the keys in their order with their raw values as they
// are, so no value is re-encoded on the way back to the file.
func rawObject(keys []string, values map[string]json.RawMessage) (json.RawMessage, error) {
	fields := make([]string, 0, len(keys))
	for _, key := range keys {
		name, err := ruleString(key)
		if err != nil {
			return nil, err
		}
		fields = append(fields, name+":"+string(values[key]))
	}
	return json.RawMessage("{" + strings.Join(fields, ",") + "}"), nil
}

func rawArray(items []json.RawMessage) json.RawMessage {
	fields := make([]string, 0, len(items))
	for _, item := range items {
		fields = append(fields, string(item))
	}
	return json.RawMessage("[" + strings.Join(fields, ",") + "]")
}

// CanonicalJSON is the value with its object keys sorted and no
// insignificant space, so a handler compares equal however an editor or
// another tool ordered its keys or indented the file; false when the
// bytes are no JSON at all.
func CanonicalJSON(raw json.RawMessage) ([]byte, bool) {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return nil, false
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	return canonical, true
}
