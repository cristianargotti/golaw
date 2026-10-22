package doctor

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// The line of a second copy of the golaw hook or skill an agent loads.
const (
	idDuplicate        = "doctor.duplicate"
	codeNoDuplicate    = "no-duplicate"
	codeDuplicateHook  = "duplicate-hook"
	codeSkillDuplicate = "skill-duplicate"
	duplicateHook      = "hook"
	duplicateSkill     = "skill"
	skillName          = "SKILL.md"
	kitName            = "golaw"
	claudeDir          = ".claude"
)

// duplicateFact is one second copy: a golaw hook command in a Claude Code
// settings file Cursor imports, or a golaw skill in a legacy directory.
type duplicateFact struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Beside string `json:"beside"`
}

// claudeSettings is the part of a Claude Code settings file Cursor imports
// as hooks (section 8.3).
type claudeSettings struct {
	Hooks map[string][]struct {
		Hooks []struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"hooks"`
	} `json:"hooks"`
}

// collectDuplicates finds what the agent would load twice: the legacy skill
// beside the one at ~/.agents/skills (section 4.5) and, for an installed
// Cursor, a golaw hook in a Claude Code settings file it imports beside its
// own hooks file.
func collectDuplicates(input Input, hooksPath string) []duplicateFact {
	if input.Agent != agentCodex && input.Agent != agentCursor {
		return nil
	}
	var out []duplicateFact
	legacy := filepath.Join(input.ConfigDir, "skills", kitName, skillName)
	if current := skillPath(input); regular(legacy) && regular(current) {
		out = append(out, duplicateFact{Kind: duplicateSkill, Path: legacy, Beside: current})
	}
	if input.Agent != agentCursor || hooksPath == "" {
		return out
	}
	for _, settings := range claudeImports(input) {
		if holdsGolawHook(settings) {
			out = append(out, duplicateFact{Kind: duplicateHook, Path: settings, Beside: hooksPath})
		}
	}
	return out
}

// skillPath is the shared skill of the scope: under the home, or under the
// repository with --scope project (D12).
func skillPath(input Input) string {
	base := input.Home
	if input.Scope == scopeProject {
		base = input.Root
	}
	return filepath.Join(base, ".agents", "skills", kitName, skillName)
}

// claudeImports are the Claude Code settings files Cursor loads hooks from.
func claudeImports(input Input) []string {
	return []string{
		filepath.Join(input.Home, claudeDir, "settings.json"),
		filepath.Join(input.Root, claudeDir, "settings.json"),
		filepath.Join(input.Root, claudeDir, "settings.local.json"),
	}
}

// regular reports whether a path is a regular file.
func regular(name string) bool {
	info, err := os.Stat(name)
	return err == nil && info.Mode().IsRegular()
}

// holdsGolawHook reports whether a settings file runs the golaw hook.
func holdsGolawHook(settings string) bool {
	data, err := os.ReadFile(settings) //nolint:gosec // the settings files Cursor imports, under the home and the repository
	if err != nil {
		return false
	}
	var document claudeSettings
	if json.Unmarshal(data, &document) != nil {
		return false
	}
	for _, groups := range document.Hooks {
		for _, group := range groups {
			for _, handler := range group.Hooks {
				if runsGolawHook(commandWords(handler.Command, handler.Args)) {
					return true
				}
			}
		}
	}
	return false
}

// commandWords splits a command line and its arguments into words at blanks
// and quotes, without a shell parser.
func commandWords(command string, args []string) []string {
	line := strings.Join(append([]string{command}, args...), " ")
	return strings.FieldsFunc(line, func(r rune) bool { return unicode.IsSpace(r) || r == '"' || r == '\'' })
}

// baseName is the last segment of a word in either separator, lower case,
// without the Windows executable suffix.
func baseName(word string) string {
	name := strings.ToLower(path.Base(strings.ReplaceAll(word, `\`, "/")))
	return strings.TrimSuffix(name, ".exe")
}

// runsGolawHook reports whether the words name the golaw binary followed by
// its hook subcommand; a lone echo or printf prints them and runs nothing.
func runsGolawHook(words []string) bool {
	if len(words) == 0 || slices.Contains([]string{"echo", "printf"}, baseName(words[0])) {
		return false
	}
	for index := range len(words) - 1 {
		if baseName(words[index]) == kitName && words[index+1] == "hook" {
			return true
		}
	}
	return false
}

// duplicateLines name every second copy, or say there is none.
func duplicateLines(f agentFacts) []Line {
	if f.Agent != agentCodex && f.Agent != agentCursor {
		return nil
	}
	lines := make([]Line, 0, len(f.Duplicates))
	for _, found := range f.Duplicates {
		line := Line{
			ID: idDuplicate, Key: f.scopeKey(), State: stateUnknown, Code: codeSkillDuplicate,
			Detail: "a golaw skill at " + found.Path + " beside " + found.Beside + ": remove the legacy copy so the agent loads one",
		}
		if found.Kind == duplicateHook {
			line.State, line.Code = stateRed, codeDuplicateHook
			line.Detail = found.Path + " holds a golaw hook command Cursor imports beside " + found.Beside +
				", so each event runs the hook twice: remove it there, the Claude Code hooks live in the plugin"
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return []Line{{ID: idDuplicate, Key: f.scopeKey(), State: stateOK, Code: codeNoDuplicate, Detail: "no second golaw hook or skill this agent loads"}}
	}
	return lines
}
