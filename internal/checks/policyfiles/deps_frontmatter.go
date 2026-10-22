package policyfiles

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// recordModules accepts an initial YAML front matter block and exact modules
// strings so prose and unrelated metadata never become dependency evidence.
func recordModules(path string, data []byte) ([]string, error) {
	block, err := recordFrontMatter(path, data)
	if err != nil || block == nil {
		return nil, err
	}
	root, err := decodeYAML(path, block)
	if err != nil {
		return nil, err
	}
	modules := child(root, "modules")
	if modules == nil {
		return nil, nil
	}
	if err := recordModuleStrings(path, modules); err != nil {
		return nil, err
	}
	return scalars(modules), nil
}

// recordFrontMatter drops a leading byte order mark, which a Windows editor
// writes, as it drops the carriage returns of converted line endings.
func recordFrontMatter(path string, data []byte) ([]byte, error) {
	lines := strings.Split(strings.TrimPrefix(string(data), "\ufeff"), "\n")
	if strings.TrimSuffix(lines[0], "\r") != "---" {
		return nil, nil
	}
	for index := 1; index < len(lines); index++ {
		if strings.TrimSuffix(lines[index], "\r") == "---" {
			// A blank opening line retains source positions without an empty YAML document.
			lines[0] = ""
			return []byte(strings.Join(lines[:index], "\n")), nil
		}
	}
	return nil, outside(path, 1, "front matter without a closing delimiter")
}

func recordModuleStrings(path string, modules *yaml.Node) error {
	if modules.Kind != yaml.SequenceNode {
		return outside(path, modules.Line, "modules that is not a sequence of strings")
	}
	for _, item := range modules.Content {
		item = resolve(item)
		if item.Kind != yaml.ScalarNode || item.Tag != "!!str" {
			return outside(path, item.Line, "a modules entry that is not a string")
		}
	}
	return nil
}
