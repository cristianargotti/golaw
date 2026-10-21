package policyfiles

import "gopkg.in/yaml.v3"

const (
	lefthookReaderCommands = "commands"
	lefthookReaderScripts  = "scripts"
)

// lefthookCommand is one command or script of a hook, kept with its line so
// a finding points at the entry a person edits.
type lefthookCommand struct {
	hook, name, run string // The git hook, the command or script name, its run text.
	line            int    // One-based line of the command or script key.
}

// readLefthook keeps commands and scripts of every hook, including disabled
// entries, because activation belongs to the developer. Its subset is local
// mappings with scalar run text, aliases and merges; external composition and
// other command shapes are stated limits, never executed or loaded.
func readLefthook(path string, data []byte) ([]lefthookCommand, error) {
	root, err := decodeYAML(path, data)
	if err != nil {
		return nil, err
	}
	var commands []lefthookCommand
	for _, hook := range entries(root) {
		if (hook.key == "extends" || hook.key == "remotes") && lefthookExternal(hook.value) {
			return nil, outside(path, hook.line, "external Lefthook configuration through "+hook.key)
		}
		if child(hook.value, lefthookReaderCommands) == nil && child(hook.value, lefthookReaderScripts) == nil && child(hook.value, buildReaderJobs) == nil {
			continue
		}
		group, err := readLefthookHook(path, hook)
		if err != nil {
			return nil, err
		}
		commands = append(commands, group...)
	}
	return commands, nil
}

func lefthookExternal(node *yaml.Node) bool {
	node = resolve(node)
	if node.Kind == yaml.ScalarNode {
		return node.Tag != "!!null" && node.Value != ""
	}
	return len(node.Content) > 0
}

func readLefthookHook(path string, hook entry) ([]lefthookCommand, error) {
	if err := buildReaderMapping(path, hook.value, "Lefthook hook", lefthookReaderCommands, lefthookReaderScripts, buildReaderJobs); err != nil {
		return nil, err
	}
	var commands []lefthookCommand
	for _, field := range entries(hook.value) {
		if field.key == buildReaderJobs && lefthookExternal(field.value) {
			return nil, outside(path, field.line, "Lefthook jobs outside commands and scripts mappings")
		}
		if field.key != lefthookReaderCommands && field.key != lefthookReaderScripts {
			continue
		}
		group, err := readLefthookEntries(path, hook.key, field)
		if err != nil {
			return nil, err
		}
		commands = append(commands, group...)
	}
	return commands, nil
}

func readLefthookEntries(path, hook string, field entry) ([]lefthookCommand, error) {
	if err := buildReaderMapping(path, field.value, "Lefthook "+field.key); err != nil {
		return nil, err
	}
	items := entries(field.value)
	commands := make([]lefthookCommand, 0, len(items))
	for _, item := range items {
		command := lefthookCommand{hook: hook, name: item.key, line: item.line}
		if field.key == lefthookReaderCommands {
			if err := buildReaderMapping(path, item.value, "Lefthook command "+item.key, buildReaderRun); err != nil {
				return nil, err
			}
			var err error
			command.run, err = buildReaderScalar(path, child(item.value, buildReaderRun), "Lefthook command run")
			if err != nil {
				return nil, err
			}
		}
		commands = append(commands, command)
	}
	return commands, nil
}
