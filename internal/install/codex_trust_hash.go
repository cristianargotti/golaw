package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// codexCommand is both the type of a command handler and the field that
// holds its command.
const (
	codexCommand        = "command"
	codexDefaultTimeout = 600
	codexContextLimit   = 2500
)

// codexHandler is a command handler as Codex deserializes it from a hooks
// file, its field names matched exactly, as Codex matches them.
type codexHandler struct {
	command, windows, status *string
	timeout, limit           *uint64
	async                    bool
}

// CodexTrustHash reproduces the hash Codex trusts a handler by: the
// SHA-256 of the compact JSON, keys sorted, of the event label, the
// matcher and the handler normalized as Codex normalizes it for goos.
// doctor and install compare it with the record, so a handler the person
// approved is never reported as untrusted and a changed one never as
// trusted. An empty matcher is an absent one.
func CodexTrustHash(event, matcher string, handler json.RawMessage, goos string) (string, error) {
	if !codexHookEvent(event) {
		return "", fmt.Errorf("codex hook event %q: %w", event, ErrSettings)
	}
	fields, err := decodeCodexHandler(handler)
	if err != nil {
		return "", err
	}
	command := *fields.command
	if goos == goosWindows && fields.windows != nil {
		command = *fields.windows
	}
	if strings.TrimSpace(command) == "" {
		return "", fmt.Errorf("codex hook command is empty: %w", ErrSettings)
	}
	identity := map[string]any{
		"event_name": codexEventLabel(event),
		"hooks":      []any{codexNormalized(fields, command)},
	}
	if matcher != "" {
		identity["matcher"] = matcher
	}
	return codexDigest(identity)
}

// codexNormalized is the handler as Codex hashes it: the timeout ten
// minutes when absent and at least one second, async false unless set, the
// status message when set and the context limit only when it is not the
// default, since Codex drops the default before it hashes.
func codexNormalized(fields codexHandler, command string) map[string]any {
	timeout := uint64(codexDefaultTimeout)
	if fields.timeout != nil {
		timeout = max(*fields.timeout, 1)
	}
	normalized := map[string]any{
		"type": codexCommand, codexCommand: command, "async": fields.async, "timeout": timeout,
	}
	if fields.status != nil {
		normalized["statusMessage"] = *fields.status
	}
	if fields.limit != nil && *fields.limit != codexContextLimit {
		normalized["additionalContextLimit"] = *fields.limit
	}
	return normalized
}

// decodeCodexHandler reads a command handler with the types Codex
// requires; a field Codex does not know is ignored, as Codex ignores it.
func decodeCodexHandler(handler json.RawMessage) (codexHandler, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(handler, &fields); err != nil || fields == nil {
		return codexHandler{}, fmt.Errorf("codex hook handler is no object: %w", ErrSettings)
	}
	var kind *string
	var decoded codexHandler
	for _, field := range []struct {
		name   string
		target any
	}{
		{"type", &kind},
		{codexCommand, &decoded.command},
		{"commandWindows", &decoded.windows},
		{"statusMessage", &decoded.status},
		{"timeout", &decoded.timeout},
		{"additionalContextLimit", &decoded.limit},
		{"async", &decoded.async},
	} {
		raw, present := fields[field.name]
		if field.name == "commandWindows" && !present {
			raw, present = fields["command_windows"]
		}
		if !present {
			continue
		}
		if err := codexField(raw, field.target); err != nil {
			return codexHandler{}, fmt.Errorf("codex hook handler field %s: %w", field.name, err)
		}
	}
	if kind == nil || *kind != codexCommand || decoded.command == nil {
		return codexHandler{}, fmt.Errorf("codex hook handler is no command: %w", ErrSettings)
	}
	return decoded, nil
}

// codexField decodes one value with the type Codex gives it: a boolean
// never null, and an integer that a TOML integer of Codex can hold.
func codexField(raw json.RawMessage, target any) error {
	if _, flag := target.(*bool); flag && bytes.Equal(raw, []byte("null")) {
		return ErrSettings
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("%w: %w", ErrSettings, err)
	}
	if number, ok := target.(**uint64); ok && *number != nil && **number > math.MaxInt64 {
		return ErrSettings
	}
	return nil
}
