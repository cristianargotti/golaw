package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
)

// Handler retains the exact exec definition required for receipt comparisons.
type Handler struct {
	Type      string   `json:"type"`
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	Timeout   int      `json:"timeout"`
	OnFailure string   `json:"onFailure"`
}

// Group retains matcher ordering because each handler has its own fail mode.
type Group struct {
	Matcher string    `json:"matcher"`
	Hooks   []Handler `json:"hooks"`
}

// Hooks prevents permissive vendor validation from hiding unknown fields.
type Hooks struct {
	Hooks map[string][]Group `json:"hooks"`
}

// ParseHooks rejects fields that the installer has not deliberately selected.
func ParseHooks(data []byte) (Hooks, error) {
	var value Hooks
	if err := strictJSON(data, &value); err != nil {
		return Hooks{}, fmt.Errorf("%w: %w", ErrHooks, err)
	}
	return value, nil
}

// ValidateHooks ensures all six required handlers use the exec form.
func ValidateHooks(data []byte) error {
	value, err := ParseHooks(data)
	if err != nil {
		return err
	}
	write := "Write|Edit|NotebookEdit"
	shell := "Bash|PowerShell"
	expected := map[string][]string{
		"PreToolUse":         {write, "Read", shell},
		"PostToolUse":        {write, shell},
		"PostToolUseFailure": {shell},
	}
	if len(value.Hooks) != len(expected) {
		return fmt.Errorf("%w: event set differs", ErrHooks)
	}
	handler := Handler{"command", kitName, []string{"hook", "claude"}, 10, "block"}
	for event, matchers := range expected {
		groups := value.Hooks[event]
		if len(groups) != len(matchers) {
			return fmt.Errorf("%w: %s group count", ErrHooks, event)
		}
		for index, matcher := range matchers {
			want := Group{Matcher: matcher, Hooks: []Handler{handler}}
			if !reflect.DeepEqual(groups[index], want) {
				return fmt.Errorf("%w: %s matcher %s", ErrHooks, event, matcher)
			}
		}
	}
	return nil
}

func strictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing JSON: %w", ErrSettings)
	}
	return nil
}
