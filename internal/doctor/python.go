package doctor

import (
	"encoding/json"
	"maps"
	"path/filepath"
	"slices"
	"strings"
)

type legacyDocument struct {
	Hooks map[string][]legacyGroup `json:"hooks"`
}

type legacyGroup struct {
	Matcher string          `json:"matcher"`
	Hooks   []legacyHandler `json:"hooks"`
}

type legacyHandler struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func pythonLines(snapshot Snapshot) []Line {
	var document legacyDocument
	if json.Unmarshal([]byte(snapshot.Settings), &document) != nil {
		return []Line{{
			ID: idPython, State: stateUnknown, Code: "python-ownership-unresolved",
			Detail: "settings cannot be inspected",
		}}
	}
	lines := make([]Line, 0, len(document.Hooks))
	for _, event := range slices.Sorted(maps.Keys(document.Hooks)) {
		for _, group := range document.Hooks[event] {
			if hasPython(group.Hooks) {
				lines = append(lines, Line{
					ID: idPython, Key: event + "/" + group.Matcher,
					State: stateUnknown, Code: "python-ownership-unresolved",
					Detail: "Python hook present; rule-family ownership is unresolved; duplicate blocking is possible",
				})
			}
		}
	}
	if len(lines) == 0 {
		lines = append(lines, Line{
			ID: idPython, State: stateOK, Code: "python-hook-absent",
			Detail: "no direct Python command hook found",
		})
	}
	return lines
}

func hasPython(handlers []legacyHandler) bool {
	for _, handler := range handlers {
		if handler.Type != "command" {
			continue
		}
		command := handler.Command
		if len(handler.Args) == 0 {
			fields := strings.Fields(command)
			if len(fields) == 0 {
				continue
			}
			command = fields[0]
		}
		name := strings.ToLower(filepath.Base(strings.Trim(command, `"'`)))
		if name == "python" || name == "python3" ||
			name == "python.exe" || name == "python3.exe" {
			return true
		}
	}
	return false
}
