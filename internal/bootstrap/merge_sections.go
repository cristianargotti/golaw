package bootstrap

import (
	"bytes"
	"slices"
	"strings"
)

type iniSection struct {
	header string
	lines  []string
}

func mergeTemplate(path string, before, rendered []byte) []byte {
	if path == ".editorconfig" {
		return mergeSections(before, rendered)
	}
	return appendTemplateLines(before, rendered)
}

func mergeSections(before, rendered []byte) []byte {
	kept := parseSections(before)
	additions := make([][]string, len(kept))
	var tail []iniSection
	changed := false
	for _, section := range parseSections(rendered) {
		index := findSection(kept, section.header)
		if index < 0 {
			tail = append(tail, section)
			continue
		}
		additions[index] = missingLines(kept[index].lines, section.lines)
		changed = changed || len(additions[index]) != 0
	}
	if !changed && len(tail) == 0 {
		return before
	}
	return writeSections(kept, additions, tail, sectionNewline(before))
}

func parseSections(data []byte) []iniSection {
	sections := []iniSection{{}}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			sections = append(sections, iniSection{header: line})
			continue
		}
		last := &sections[len(sections)-1]
		last.lines = append(last.lines, line)
	}
	return sections
}

func findSection(sections []iniSection, header string) int {
	return slices.IndexFunc(sections, func(section iniSection) bool {
		return strings.TrimSpace(section.header) == strings.TrimSpace(header)
	})
}

func missingLines(before, rendered []string) []string {
	current := make(map[string]int)
	for _, line := range before {
		current[strings.TrimSpace(line)]++
	}
	var missing []string
	for _, line := range rendered {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if current[trimmed] > 0 {
			current[trimmed]--
			continue
		}
		missing = append(missing, line)
	}
	return missing
}

func writeSections(kept []iniSection, additions [][]string, tail []iniSection, newline string) []byte {
	var after []byte
	for index, section := range kept {
		after = writeSection(after, section, additions[index], newline)
	}
	return writeTail(after, tail, newline)
}

func writeSection(after []byte, section iniSection, additions []string, newline string) []byte {
	if section.header != "" {
		after = append(after, section.header+newline...)
	}
	end := len(section.lines)
	for end > 0 && strings.TrimSpace(section.lines[end-1]) == "" {
		end--
	}
	for _, line := range slices.Concat(section.lines[:end], additions, section.lines[end:]) {
		after = append(after, line+newline...)
	}
	return after
}

func writeTail(after []byte, tail []iniSection, newline string) []byte {
	for _, section := range tail {
		lastLine := bytes.TrimSuffix(after, []byte(newline))
		lastLine = lastLine[bytes.LastIndexByte(lastLine, '\n')+1:]
		if len(bytes.TrimSpace(lastLine)) != 0 {
			after = append(after, newline...)
		}
		after = writeSection(after, section, nil, newline)
	}
	return after
}

func sectionNewline(before []byte) string {
	return policyNewline(before)
}
