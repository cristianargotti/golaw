package coexist

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// The tool names of the agent, as the hook matchers spell them: every tool
// the kit's own hooks answer, so a gate hook on any of them is a second
// verdict on an event the kit arms.
const (
	toolWrite      = "Write"
	toolEdit       = "Edit"
	toolNotebook   = "NotebookEdit"
	toolRead       = "Read"
	toolBash       = "Bash"
	toolPowerShell = "PowerShell"
)

// exactMatcher is what the agent compares as exact tool names, one or a
// list split by "|" or ","; any other character makes the matcher an
// unanchored regular expression, which the detection does not evaluate.
var exactMatcher = regexp.MustCompile(`^[A-Za-z0-9_ ,|-]*$`)

// settings is the shape of the user settings the detection reads.
type settings struct {
	Hooks map[string][]matcher `json:"hooks"`
}

// matcher is one hook group: a tool matcher and its commands.
type matcher struct {
	Matcher string `json:"matcher"`
	Hooks   []hook `json:"hooks"`
}

// hook is one command of a group: a shell command line, or an executable
// with its argument vector when args is set (the exec form, no shell).
type hook struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// registersGate reports whether one of the group's commands runs the gate.
func (m matcher) registersGate() bool {
	return slices.ContainsFunc(m.Hooks, hook.runsGate)
}

// runsGate reports whether the hook may run the gate: a word of its command
// line, the exec form's arguments included, names the script as a path, so
// a script whose name only ends in it is another hook, and the line is not
// a lone echo or printf, which prints the name and runs nothing. With no
// shell parser any other mention counts, a wrapped run among them, because
// a wrong refusal costs less than a second blocking verdict.
func (h hook) runsGate() bool {
	line := strings.Join(append([]string{h.Command}, h.Args...), " ")
	words := strings.FieldsFunc(line, wordBreak)
	if len(words) == 0 || printsOnly(words[0], line) {
		return false
	}
	return slices.ContainsFunc(words, func(word string) bool { return filepath.Base(word) == gateScript })
}

// wordBreak ends a word of a command line: a blank, a quote, or a
// character that joins, groups or redirects commands.
func wordBreak(r rune) bool {
	return unicode.IsSpace(r) || strings.ContainsRune("\"'`;&|()<>", r)
}

// printsOnly reports whether the line is a lone echo or printf: it prints
// its words and joins no other command that could run the script.
func printsOnly(program, line string) bool {
	name := filepath.Base(program)
	return (name == "echo" || name == "printf") && !strings.ContainsAny(line, ";&|`$()\n")
}

// markEvents records the normalized events a gate group of an agent event
// covers: a write tool, the read tool or the shell tool, before or after. A
// matcher the detection cannot read as tool names is an error, because an
// event read as no tool would let the kit arm a second verdict on it.
func markEvents(registered map[string]bool, agentEvent, matcherText string) error {
	post := agentEvent == "PostToolUse"
	if !post && agentEvent != "PreToolUse" {
		return nil
	}
	tools, exact := matchedTools(matcherText)
	if !exact {
		return fmt.Errorf("%w: %s: the %s matcher %q is a regular expression the kit does not evaluate",
			ErrDetect, settingsFile, agentEvent, matcherText)
	}
	for _, tool := range tools {
		switch tool {
		case toolWrite, toolEdit, toolNotebook:
			registered[pick(post, EventPostWrite, EventWrite)] = true
		case toolRead:
			if !post {
				registered[EventRead] = true
			}
		case toolBash, toolPowerShell:
			registered[pick(post, EventPostShell, EventShell)] = true
		}
	}
	return nil
}

// matchedTools lists the tools a matcher names, false for a regular
// expression. The agent runs a group without a matcher, or with "*", for
// every tool, so those forms name every tool the kit answers.
func matchedTools(matcherText string) ([]string, bool) {
	text := strings.TrimSpace(matcherText)
	if text == "" || text == "*" {
		return []string{toolWrite, toolEdit, toolNotebook, toolRead, toolBash, toolPowerShell}, true
	}
	if !exactMatcher.MatchString(text) {
		return nil, false
	}
	tools := strings.FieldsFunc(text, func(r rune) bool { return r == '|' || r == ',' })
	for i := range tools {
		tools[i] = strings.TrimSpace(tools[i])
	}
	return tools, true
}

func pick(post bool, postEvent, preEvent string) string {
	if post {
		return postEvent
	}
	return preEvent
}
