package assets

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

const (
	ciRoot  = "document"
	ciAlias = "alias is unsupported"
)

func ciDocument(data []byte) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, ciError(nil, ciRoot, "expected mapping")
		}
		return nil, fmt.Errorf("%w: %w", ciError(&document, ciRoot, "decode"), err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ciError(&extra, ciRoot, "decode"), err)
		}
		return nil, ciError(&extra, ciRoot, "multiple documents are unsupported")
	}
	return document.Content[0], nil
}

func ciMapping(node *yaml.Node, location string) (map[string]*yaml.Node, error) {
	if node.Kind == yaml.AliasNode {
		return nil, ciError(node, location, ciAlias)
	}
	if node.Kind != yaml.MappingNode {
		return nil, ciError(node, location, "expected mapping")
	}
	fields := make(map[string]*yaml.Node, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i]
		if key.Tag == "!!merge" {
			return nil, ciError(key, location, "merge key is unsupported")
		}
		name, err := ciScalar(key, location+" key")
		if err != nil {
			return nil, err
		}
		if _, exists := fields[name]; exists {
			return nil, ciError(key, location, "duplicate key "+name)
		}
		fields[name] = node.Content[i+1]
	}
	return fields, nil
}

func ciScalar(node *yaml.Node, location string) (string, error) {
	if node.Kind == yaml.AliasNode {
		return "", ciError(node, location, ciAlias)
	}
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return "", ciError(node, location, "expected string")
	}
	return node.Value, nil
}

func ciError(node *yaml.Node, location, reason string) error {
	line := 1
	if node != nil && node.Line > 0 {
		line = node.Line
	}
	return fmt.Errorf("%w: %s line %d: %s", ErrRunSteps, location, line, reason)
}
