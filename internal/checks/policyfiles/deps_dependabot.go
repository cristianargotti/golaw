package policyfiles

import (
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	dependabotStringTag = "!!str"
	dependabotGo        = "gomod"
	cooldownInclude     = "include"
)

type dependabot struct {
	updates []dependabotUpdate
}

type dependabotUpdate struct {
	ecosystem string
	days      map[string]int
	include   []string
	exclude   []string
	line      int
}

// readDependabot reads the subset the cooldown verdict depends on: version
// 2, the updates sequence, each package-ecosystem string and, for gomod and
// github-actions, a cooldown mapping of integer days with string include
// and exclude lists. Everything else is skipped, so unrelated ecosystems and
// configuration fields cannot impose Go-specific limits.
func readDependabot(path string, data []byte) (dependabot, error) {
	root, err := decodeYAML(path, data)
	if err != nil {
		return dependabot{}, err
	}
	if err := dependabotUnique(path, root, "version", "updates"); err != nil {
		return dependabot{}, err
	}
	version := child(root, "version")
	var number int
	if version == nil || version.Tag != "!!int" || version.Decode(&number) != nil || number != 2 {
		return dependabot{}, outside(path, dependabotLine(version), "Dependabot version other than 2")
	}
	updates := child(root, "updates")
	if updates == nil || updates.Kind != yaml.SequenceNode {
		return dependabot{}, outside(path, dependabotLine(updates), "updates that is not a sequence")
	}
	var result dependabot
	for _, node := range updates.Content {
		update, err := readDependabotUpdate(path, node)
		if err != nil {
			return dependabot{}, err
		}
		if update.ecosystem != "" {
			result.updates = append(result.updates, update)
		}
	}
	return result, nil
}

func readDependabotUpdate(path string, node *yaml.Node) (dependabotUpdate, error) {
	line := node.Line
	node = resolve(node)
	if node.Kind != yaml.MappingNode {
		return dependabotUpdate{}, outside(path, line, "an updates entry that is not a mapping")
	}
	if err := dependabotUnique(path, node, "package-ecosystem"); err != nil {
		return dependabotUpdate{}, err
	}
	ecosystem := child(node, "package-ecosystem")
	if ecosystem == nil {
		return dependabotUpdate{}, outside(path, line, "an updates entry without package-ecosystem")
	}
	if ecosystem.Kind != yaml.ScalarNode || ecosystem.Tag != dependabotStringTag {
		return dependabotUpdate{}, outside(path, ecosystem.Line, "package-ecosystem that is not a string")
	}
	if ecosystem.Value != dependabotGo && ecosystem.Value != "github-actions" {
		return dependabotUpdate{}, nil
	}
	if err := dependabotUnique(path, node, "cooldown"); err != nil {
		return dependabotUpdate{}, err
	}
	update := dependabotUpdate{ecosystem: ecosystem.Value, line: line, days: make(map[string]int)}
	cooldown := child(node, "cooldown")
	if cooldown == nil {
		return update, nil
	}
	if cooldown.Kind != yaml.MappingNode {
		return dependabotUpdate{}, outside(path, cooldown.Line, "cooldown that is not a mapping")
	}
	if err := readCooldown(path, cooldown, &update); err != nil {
		return dependabotUpdate{}, err
	}
	return update, nil
}

func readCooldown(path string, node *yaml.Node, update *dependabotUpdate) error {
	keys := []string{cooldownDefault, cooldownInclude, "exclude"}
	if update.ecosystem == dependabotGo {
		keys = append(keys, cooldownMajor, cooldownMinor, cooldownPatch)
	}
	if err := dependabotUnique(path, node, keys...); err != nil {
		return err
	}
	for _, field := range entries(node) {
		if err := readCooldownField(path, field, update); err != nil {
			return err
		}
	}
	return nil
}

func readCooldownField(path string, field entry, update *dependabotUpdate) error {
	value := resolve(field.value)
	switch field.key {
	case cooldownDefault, cooldownMajor, cooldownMinor, cooldownPatch:
		if update.ecosystem == "github-actions" && strings.HasPrefix(field.key, "semver-") {
			return nil
		}
		var days int
		if value.Kind != yaml.ScalarNode || value.Tag != "!!int" || value.Decode(&days) != nil {
			return outside(path, field.line, "cooldown."+field.key+" that is not an integer")
		}
		update.days[field.key] = days
	case cooldownInclude, "exclude":
		names, err := cooldownNames(path, field.key, value)
		if err != nil {
			return err
		}
		if field.key == cooldownInclude {
			update.include = names
		} else {
			update.exclude = names
		}
	}
	return nil
}

func cooldownNames(path, key string, node *yaml.Node) ([]string, error) {
	if node.Kind != yaml.SequenceNode {
		return nil, outside(path, node.Line, "cooldown."+key+" that is not a sequence of strings")
	}
	for _, item := range node.Content {
		item = resolve(item)
		if item.Kind != yaml.ScalarNode || item.Tag != dependabotStringTag {
			return nil, outside(path, item.Line, "a cooldown."+key+" entry that is not a string")
		}
	}
	return scalars(node), nil
}

func dependabotLine(node *yaml.Node) int {
	if node == nil {
		return 1
	}
	return max(1, node.Line)
}

func dependabotUnique(path string, node *yaml.Node, keys ...string) error {
	seen := make(map[string]bool)
	for _, field := range entries(node) {
		if !slices.Contains(keys, field.key) {
			continue
		}
		if seen[field.key] {
			return outside(path, field.line, "duplicate "+field.key+" key")
		}
		seen[field.key] = true
	}
	return nil
}
