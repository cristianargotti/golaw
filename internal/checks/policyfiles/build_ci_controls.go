package policyfiles

import (
	"slices"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

const demandShellKey = "shell"

// demandDisabled recognizes the one condition known without evaluation,
// a single literal if: false, which runs nothing and so proves nothing.
func demandDisabled(node *yaml.Node) bool {
	count := 0
	for _, item := range entries(node) {
		if item.key == "if" {
			count++
		}
	}
	condition := child(node, "if")
	return count == 1 && condition != nil && condition.Kind == yaml.ScalarNode && condition.Value == demandFalse
}

// demandControls accepts only the controls whose effect is certain, a true
// condition and a false continue-on-error, because any other value could
// skip the make line or hide its failure.
func demandControls(file string, root, job, step *yaml.Node) error {
	for _, node := range []*yaml.Node{job, step} {
		if err := buildReaderMapping(file, node, "Actions execution controls", "if", "continue-on-error", "defaults", demandDirectoryKey, demandShellKey); err != nil {
			return err
		}
		for _, control := range []struct{ key, safe string }{{"if", yamlTrue}, {"continue-on-error", demandFalse}} {
			value := child(node, control.key)
			if value != nil && (value.Kind != yaml.ScalarNode || value.Value != control.safe) {
				return outside(file, value.Line, "Actions "+control.key+": "+value.Value)
			}
		}
	}
	return demandDefaults(file, root, job, step)
}

// demandDefaults holds the make line to the root of the repository and to a
// POSIX shell, because another directory runs another Makefile and the
// Windows shells do not stop on a failed native command.
func demandDefaults(file string, root, job, step *yaml.Node) error {
	for _, setting := range []struct {
		key  string
		safe []string
	}{
		{demandDirectoryKey, []string{".", "./"}},
		{demandShellKey, []string{"bash", "sh"}},
	} {
		value, err := demandSetting(file, root, job, step, setting.key)
		if err != nil {
			return err
		}
		if value != nil && (value.Kind != yaml.ScalarNode || !slices.Contains(setting.safe, value.Value)) {
			return outside(file, value.Line, "Actions "+setting.key+": "+value.Value)
		}
		if setting.key == demandShellKey && value == nil {
			if windows := demandWindows(child(job, "runs-on")); windows != nil {
				return outside(file, windows.Line, "Actions default shell on Windows runs-on: "+windows.Value)
			}
		}
	}
	return nil
}

// demandWindows answers the runs-on label that selects a Windows runner,
// whose default shell is PowerShell rather than bash.
func demandWindows(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.MappingNode {
		node = child(node, "labels")
	}
	nodes := []*yaml.Node{node}
	if node != nil && node.Kind == yaml.SequenceNode {
		nodes = node.Content
	}
	for _, item := range nodes {
		item = resolve(item)
		if item == nil || item.Kind != yaml.ScalarNode {
			continue
		}
		label := strings.ToLower(item.Value)
		if label == "windows" || strings.HasPrefix(label, "windows-") {
			return item
		}
	}
	return nil
}

// demandSetting answers the effective value of a run default as Actions
// resolves it: the file's defaults, then the job's, then the step's own.
func demandSetting(file string, root, job, step *yaml.Node, key string) (*yaml.Node, error) {
	var directory *yaml.Node
	for _, node := range []*yaml.Node{root, job} {
		for _, part := range []string{"defaults", "run", key} {
			if err := buildReaderMapping(file, node, "Actions execution defaults", part); err != nil {
				return nil, err
			}
			node = child(node, part)
		}
		if node != nil {
			directory = node
		}
	}
	if value := child(step, key); value != nil {
		directory = value
	}
	return directory, nil
}

// Execution controls matter only when the text could invoke make.
func demandRelevant(run string) bool {
	words := strings.FieldsFunc(run, func(char rune) bool {
		return unicode.IsSpace(char) || strings.ContainsRune("&;|'\"", char)
	})
	return slices.Contains(words, ciMakeCommand)
}

// demandMakeEnvironment recognizes make run under assignments such as
// MAKEFLAGS=-i, whose effect on the outcome the reader does not judge.
func demandMakeEnvironment(line string) bool {
	for i, word := range demandWords(line) {
		if word == ciMakeCommand {
			return i > 0
		}
		if !demandAssignment(word) {
			return false
		}
	}
	return false
}

func demandAssignment(word string) bool {
	name, _, found := strings.Cut(word, "=")
	if !found || name == "" {
		return false
	}
	for i, char := range name {
		if char == '_' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || i > 0 && char >= '0' && char <= '9' {
			continue
		}
		return false
	}
	return true
}

// Quoted whitespace belongs to an assignment value whose effects remain unknown.
func demandWords(line string) []string {
	var words []string
	var word []rune
	var quote rune
	for _, char := range line {
		if quote == 0 && unicode.IsSpace(char) {
			if len(word) != 0 {
				words = append(words, string(word))
				word = word[:0]
			}
			continue
		}
		if char == quote {
			quote = 0
		} else if quote == 0 && strings.ContainsRune("'\"", char) {
			quote = char
		}
		word = append(word, char)
	}
	if len(word) != 0 {
		words = append(words, string(word))
	}
	return words
}
