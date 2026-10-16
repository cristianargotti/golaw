package cicheck_test

import (
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
)

func document(data []byte) (*yaml.Node, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("decode asset: %w", err)
	}
	if len(node.Content) != 1 {
		return nil, fmt.Errorf("decode asset: %w", model.ErrUsage)
	}
	return node.Content[0], nil
}

func at(node *yaml.Node, path ...string) *yaml.Node {
	for _, key := range path {
		if node == nil {
			return nil
		}
		switch node.Kind {
		case yaml.SequenceNode:
			node = atSequence(node, key)
		case yaml.MappingNode:
			node = atMapping(node, key)
		case yaml.DocumentNode, yaml.ScalarNode, yaml.AliasNode:
			return nil
		default:
			return nil
		}
	}
	return node
}

func atSequence(node *yaml.Node, key string) *yaml.Node {
	index, err := strconv.Atoi(key)
	if err != nil || index < 0 || index >= len(node.Content) {
		return nil
	}
	return node.Content[index]
}

func atMapping(node *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func value(node *yaml.Node) string {
	if node == nil {
		return ""
	}
	return node.Value
}

func children(node *yaml.Node) []*yaml.Node {
	if node == nil {
		return nil
	}
	return node.Content
}

func named(node *yaml.Node, key string) []*yaml.Node {
	if node == nil {
		return nil
	}
	var result []*yaml.Node
	if node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				result = append(result, node.Content[i+1])
			}
		}
	}
	for _, child := range node.Content {
		result = append(result, named(child, key)...)
	}
	return result
}
