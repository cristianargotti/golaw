package surface

import (
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// resolve follows an alias to the node it names, so an anchored resource
// reads as written.
func resolve(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}

// mapValue answers the value of a key of a mapping, nil when the node is
// no mapping or lacks the key.
func mapValue(n *yaml.Node, key string) *yaml.Node {
	n = resolve(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return resolve(n.Content[i+1])
		}
	}
	return nil
}

// scalarValue answers the text of a scalar, empty for any other node.
func scalarValue(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

// refers answers the logical id a Bucket names by !Ref or by a Ref
// mapping; any other form, a literal name included, refers to no resource
// of the template.
func refers(n *yaml.Node) (string, bool) {
	if n == nil {
		return "", false
	}
	if n.Kind == yaml.ScalarNode {
		return n.Value, n.Tag == "!Ref" && n.Value != ""
	}
	id := scalarValue(mapValue(n, "Ref"))
	return id, id != ""
}

// statements answers the statements of a policy document: a sequence, or
// one mapping.
func statements(n *yaml.Node) []*yaml.Node {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.MappingNode {
		return []*yaml.Node{n}
	}
	if n.Kind != yaml.SequenceNode {
		return nil
	}
	out := make([]*yaml.Node, 0, len(n.Content))
	for _, s := range n.Content {
		out = append(out, resolve(s))
	}
	return out
}

// denies reports a statement that denies every action when the request
// does not travel over HTTPS.
func denies(statement *yaml.Node) bool {
	return scalarValue(mapValue(statement, "Effect")) == "Deny" &&
		actionAll(mapValue(statement, "Action")) &&
		insecureFalse(mapValue(mapValue(mapValue(statement, "Condition"), "Bool"), "aws:SecureTransport"))
}

// actionAll reports an Action of every action: a scalar, or a sequence
// that holds one.
func actionAll(n *yaml.Node) bool {
	return holds(n, func(v string) bool { return slices.Contains([]string{"*", "s3:*"}, v) })
}

// insecureFalse reports a condition value of false in any case.
func insecureFalse(n *yaml.Node) bool {
	return holds(n, func(v string) bool { return strings.EqualFold(v, "false") })
}

// holds reports a scalar, or a sequence holding a scalar, whose text
// matches.
func holds(n *yaml.Node, match func(string) bool) bool {
	if n == nil {
		return false
	}
	if n.Kind == yaml.ScalarNode {
		return match(n.Value)
	}
	if n.Kind != yaml.SequenceNode {
		return false
	}
	return slices.ContainsFunc(n.Content, func(item *yaml.Node) bool {
		item = resolve(item)
		return item.Kind == yaml.ScalarNode && match(item.Value)
	})
}
