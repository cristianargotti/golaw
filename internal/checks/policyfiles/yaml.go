package policyfiles

import (
	"bytes"
	"errors"
	"io"

	"gopkg.in/yaml.v3"
)

// mergeTag is the tag yaml.v3 gives the key of a YAML 1.1 merge.
const mergeTag = "!!merge"

// keyVersion is the schema key that the Dependabot and the golangci-lint
// readers both judge, so one constant keeps the two readers in step.
const keyVersion = "version"

// entry is one key of a mapping with its value and the line of the key.
type entry struct {
	key   string
	value *yaml.Node
	line  int
}

// decodeYAML parses the one document of a YAML configuration and answers
// its root mapping with every merge key expanded, so a reader sees the
// mapping a tool sees. An empty document is an empty mapping; a root that
// is not a mapping or a second document is a stated limit; a syntax error
// names the file.
func decodeYAML(path string, data []byte) (*yaml.Node, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); errors.Is(err, io.EOF) {
		return &yaml.Node{Kind: yaml.MappingNode}, nil
	} else if err != nil {
		return nil, parseError(path, err)
	}
	var next yaml.Node
	if err := dec.Decode(&next); !errors.Is(err, io.EOF) {
		return nil, outside(path, next.Line, "a second YAML document")
	}
	if len(doc.Content) == 0 {
		return &yaml.Node{Kind: yaml.MappingNode}, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, outside(path, root.Line, "a YAML root that is not a mapping")
	}
	expand(root)
	return root, nil
}

// expand rewrites every mapping under node to hold the pairs its merge keys
// bring after its own pairs, an own key winning, as YAML 1.1 merges them.
// An alias is never walked, so an anchor that refers to itself cannot loop.
func expand(node *yaml.Node) {
	for _, c := range node.Content {
		expand(c)
	}
	if node.Kind != yaml.MappingNode {
		return
	}
	var own, merged []*yaml.Node
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if key.Tag == mergeTag {
			merged = append(merged, sources(value)...)
			continue
		}
		own = append(own, key, value)
	}
	for _, m := range merged {
		for i := 0; i+1 < len(m.Content); i += 2 {
			if !hasKey(own, m.Content[i].Value) {
				own = append(own, m.Content[i], m.Content[i+1])
			}
		}
	}
	node.Content = own
}

// sources answers the mappings a merge value names: one alias or a
// sequence of aliases, each resolved.
func sources(value *yaml.Node) []*yaml.Node {
	value = resolve(value)
	if value.Kind == yaml.MappingNode {
		return []*yaml.Node{value}
	}
	var out []*yaml.Node
	for _, item := range value.Content {
		if item = resolve(item); item.Kind == yaml.MappingNode {
			out = append(out, item)
		}
	}
	return out
}

func hasKey(pairs []*yaml.Node, key string) bool {
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i].Value == key {
			return true
		}
	}
	return false
}

// resolve follows an alias to its anchor.
func resolve(node *yaml.Node) *yaml.Node {
	for node != nil && node.Kind == yaml.AliasNode {
		node = node.Alias
	}
	return node
}

// child walks mapping keys from a node and answers the value under the
// last one, aliases followed; nil when a key is absent or a node on the
// way is not a mapping.
func child(node *yaml.Node, keys ...string) *yaml.Node {
	for _, key := range keys {
		node = resolve(node)
		if node == nil || node.Kind != yaml.MappingNode {
			return nil
		}
		var next *yaml.Node
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				next = node.Content[i+1]
			}
		}
		node = next
	}
	return resolve(node)
}

// scalars answers the values of a sequence of scalars, flow or block, and
// nil for any other node.
func scalars(node *yaml.Node) []string {
	node = resolve(node)
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil
	}
	out := make([]string, 0, len(node.Content))
	for _, item := range node.Content {
		if item = resolve(item); item.Kind == yaml.ScalarNode {
			out = append(out, item.Value)
		}
	}
	return out
}

// entries answers the keys of a mapping in file order, each with its value
// and the line of the key; nil for any other node.
func entries(node *yaml.Node) []entry {
	node = resolve(node)
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	out := make([]entry, 0, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		out = append(out, entry{key: node.Content[i].Value, value: node.Content[i+1], line: node.Content[i].Line})
	}
	return out
}
