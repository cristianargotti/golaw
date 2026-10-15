package coexist

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Where the Python gate registers itself on the machine: the Claude Code
// user settings carry its hooks, the registry beside them lists the
// repositories it governs and the flags of its families.
const (
	agentDir     = ".claude"
	settingsFile = "settings.json"
	registryFile = "pcc-standards.json"
	gateScript   = "pcc_gate.py"
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

// Detection is what the machine and the repository say about the gate.
type Detection struct {
	Registered map[string]bool // the events a gate hook answers
	Governs    bool            // the repository is in the gate's registry
	Released   map[string]bool // the families the registry switched off
}

// settings is the shape of the user settings the detection reads.
type settings struct {
	Hooks map[string][]matcher `json:"hooks"`
}

// matcher is one hook group: a tool matcher and its commands.
type matcher struct {
	Matcher string `json:"matcher"`
	Hooks   []hook `json:"hooks"`
}

// hook is one command of a group.
type hook struct {
	Command string `json:"command"`
}

// registry is the shape of the gate registry the detection reads: the
// governed repositories and one switch per shared family, absent or true
// while the gate keeps the family.
type registry struct {
	Repos      []string `json:"repos"`
	Text       *bool    `json:"text_checks"`
	Paths      *bool    `json:"paths_checks"`
	Sizes      *bool    `json:"sizes_checks"`
	SecretRead *bool    `json:"secret_read_checks"`
	Shell      *bool    `json:"shell_checks"`
}

// Detect reads the registration of the gate under home and answers whether
// it governs the repository at root. A missing file means no gate; a file
// that cannot be decoded is an error.
func Detect(home, root string) (Detection, error) {
	d := Detection{Registered: map[string]bool{}, Released: map[string]bool{}}
	var s settings
	if err := decode(SettingsPath(home), &s); err != nil {
		return Detection{}, err
	}
	for event, groups := range s.Hooks {
		for _, group := range groups {
			if !group.registersGate() {
				continue
			}
			if err := markEvents(d.Registered, event, group.Matcher); err != nil {
				return Detection{}, err
			}
		}
	}
	var r registry
	if err := decode(RegistryPath(home), &r); err != nil {
		return Detection{}, err
	}
	d.Governs = governs(home, r.Repos, root)
	d.Released = r.released()
	return d, nil
}

// SettingsPath is the user settings file of the agent under a home, where
// the gate registers its hooks.
func SettingsPath(home string) string { return filepath.Join(home, agentDir, settingsFile) }

// RegistryPath is the registry of the gate beside the settings: the
// repositories it governs and the switches of its families.
func RegistryPath(home string) string { return filepath.Join(home, agentDir, registryFile) }

// decode reads a JSON file into v; a missing file leaves v untouched.
func decode(path string, v any) error {
	data, err := os.ReadFile(path) //nolint:gosec // the path is a registration file under the person's home
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrDetect, filepath.Base(path), err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%w: %s: %w", ErrDetect, filepath.Base(path), err)
	}
	return nil
}

// registersGate reports whether one of the group's commands is the gate.
func (m matcher) registersGate() bool {
	for _, h := range m.Hooks {
		if strings.Contains(h.Command, gateScript) {
			return true
		}
	}
	return false
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

// pick chooses the post or the pre event.
func pick(post bool, postEvent, preEvent string) string {
	if post {
		return postEvent
	}
	return preEvent
}

// governs reports whether root is one of the governed repositories or a
// directory inside one. Both sides take the form the gate compares, so a
// relative, linked or home-relative path matches as it does for the gate.
func governs(home string, repos []string, root string) bool {
	target, ok := realPath(home, root)
	if !ok {
		return false
	}
	for _, repo := range repos {
		governed, ok := realPath(home, repo)
		if !ok {
			continue
		}
		if target == governed || strings.HasPrefix(target, governed+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// realPath is the absolute path with every link resolved, a leading ~
// standing for the home: the realpath of expanduser the gate applies to
// its registry and to the path it judges.
func realPath(home, path string) (string, bool) {
	if rest, ok := strings.CutPrefix(path, "~"); ok && (rest == "" || os.IsPathSeparator(rest[0])) {
		path = filepath.Join(home, rest)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	resolved, err := filepath.EvalSymlinks(abs)
	return resolved, err == nil
}

// released lists the families whose switch is explicitly off.
func (r registry) released() map[string]bool {
	flags := map[string]*bool{
		FamilyText: r.Text, FamilyPaths: r.Paths, FamilySizes: r.Sizes,
		FamilySecretRead: r.SecretRead, FamilyShell: r.Shell,
	}
	out := map[string]bool{}
	for family, flag := range flags {
		if flag != nil && !*flag {
			out[family] = true
		}
	}
	return out
}

// owns reports whether the gate keeps the family on this repository.
func (d Detection) owns(family string) bool {
	if !d.Governs || d.Released[family] {
		return false
	}
	return slices.ContainsFunc(judged()[family], func(event string) bool { return d.Registered[event] })
}

// summary is the first line of the table.
func (d Detection) summary() string {
	events := make([]string, 0, len(d.Registered))
	for _, event := range []string{EventWrite, EventRead, EventShell, EventPostWrite, EventPostShell} {
		if d.Registered[event] {
			events = append(events, event)
		}
	}
	if len(events) == 0 {
		return "gate: no hook registered on this machine"
	}
	scope := "does not govern this repository"
	if d.Governs {
		scope = "governs this repository"
	}
	return "gate: registered for " + strings.Join(events, ", ") + "; " + scope
}
