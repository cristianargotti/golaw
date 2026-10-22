package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/install"
)

// hookTimeout is the timeout every installer of M4 writes, in seconds,
// above the internal budget of the hook (D6).
const hookTimeout = 10

// exactList is a Codex matcher that lists tool names: only letters,
// digits, underscores and bars (survey section 2.1).
var exactList = regexp.MustCompile(`^[A-Za-z0-9_|]+$`)

// m4Handler is one handler of an installed Codex or Cursor hooks file, in
// the fields the harness judges; Cursor keeps the matcher on the handler.
type m4Handler struct {
	Type           string `json:"type"`
	Command        string `json:"command"`
	CommandWindows string `json:"commandWindows"`
	Matcher        string `json:"matcher"`
	Timeout        int    `json:"timeout"`
	FailClosed     *bool  `json:"failClosed"`
	OnFailure      string `json:"onFailure"`
	Async          bool   `json:"async"`
}

// m4Entry is one installed handler with its event, its place in the file
// and the exact JSON written: Group is the index of its group (of the
// handler itself in a Cursor file) and Index the index in that group.
type m4Entry struct {
	Event        string
	Group, Index int
	Matcher      string
	Handler      m4Handler
	Raw          json.RawMessage
}

// m4System is the operating system the rows replay for.
func (s *suite) m4System() string {
	if s.m2OS != "" {
		return s.m2OS
	}
	return runtime.GOOS
}

// m4ReadEntries lists every handler of a hooks file, events in name order
// and handlers in file order.
func m4ReadEntries(path, agent string) ([]m4Entry, error) {
	body, err := readFile(path)
	if err != nil {
		return nil, err
	}
	var document struct {
		Hooks map[string][]json.RawMessage `json:"hooks"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, fmt.Errorf(decodeFormat, path, err)
	}
	var entries []m4Entry
	for _, event := range slices.Sorted(maps.Keys(document.Hooks)) {
		for i, raw := range document.Hooks[event] {
			found, err := m4Group(agent, event, i, raw)
			if err != nil {
				return nil, fmt.Errorf("decode %s of %s: %w", event, path, err)
			}
			entries = append(entries, found...)
		}
	}
	return entries, nil
}

// m4Group reads one element of an event's list: a Codex group of
// handlers, or a Cursor handler.
func m4Group(agent, event string, group int, raw json.RawMessage) ([]m4Entry, error) {
	if agent == agentCursor {
		var handler m4Handler
		if err := json.Unmarshal(raw, &handler); err != nil {
			return nil, fmt.Errorf("decode a handler: %w", err)
		}
		return []m4Entry{{Event: event, Group: group, Matcher: handler.Matcher, Handler: handler, Raw: raw}}, nil
	}
	var decoded struct {
		Matcher string            `json:"matcher"`
		Hooks   []json.RawMessage `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("decode a group: %w", err)
	}
	entries := make([]m4Entry, 0, len(decoded.Hooks))
	for j, item := range decoded.Hooks {
		var handler m4Handler
		if err := json.Unmarshal(item, &handler); err != nil {
			return nil, fmt.Errorf("decode a handler: %w", err)
		}
		entries = append(entries, m4Entry{Event: event, Group: group, Index: j, Matcher: decoded.Matcher, Handler: handler, Raw: item})
	}
	return entries, nil
}

// m4Selects reads a matcher the way its host does: empty or * selects
// every tool; a Codex pattern of names only is an exact list, any other
// pattern a regular expression searched unanchored (survey section 2.1);
// Cursor documents a regular expression (survey section 8.1).
func m4Selects(agent, matcher, tool string) (bool, error) {
	if matcher == "" || matcher == "*" {
		return true, nil
	}
	if agent == agentCodex && exactList.MatchString(matcher) {
		return slices.Contains(strings.Split(matcher, "|"), tool), nil
	}
	pattern, err := regexp.Compile(matcher)
	if err != nil {
		return false, fmt.Errorf("compile the matcher %q: %w", matcher, err)
	}
	return pattern.MatchString(tool), nil
}

// m4Command is the command string the host runs for a handler: Codex
// replaces command with commandWindows on Windows (survey section 4), and
// the Cursor command is itself the launcher there (D13).
func m4Command(agent, goos string, handler m4Handler) string {
	if agent == agentCodex && goos == goosWindows {
		return handler.CommandWindows
	}
	return handler.Command
}

// m4Names reports whether a command the installer wrote names the binary,
// compared as files, because a temporary directory reached through a link
// spells one file two ways.
func m4Names(command, binary string) bool {
	named, ok := install.CommandBinary(command)
	if !ok {
		return false
	}
	left, leftErr := os.Stat(named)
	right, rightErr := os.Stat(binary)
	return leftErr == nil && rightErr == nil && os.SameFile(left, right)
}
