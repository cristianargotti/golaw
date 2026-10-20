package bootstrap

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

type policyEdit struct {
	start, end int
	value      []byte
}

const managedKey = "managed"

// ManagedPolicy preserves reviewed bytes outside the two values init owns.
func ManagedPolicy(current []byte, version string, managed []policy.ManagedFile) ([]byte, error) {
	decoded, err := policy.Decode(current)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: managed policy: %w", err)
	}
	var document yaml.Node
	if err = yaml.Unmarshal(current, &document); err != nil {
		return nil, fmt.Errorf("bootstrap: managed policy nodes: %w", err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, unsupportedManaged("policy", "expected a mapping")
	}
	root := document.Content[0]
	versionEdit, err := replaceVersion(current, root, version)
	if err != nil {
		return nil, err
	}
	_, existing := mappingValue(root, managedKey)
	managedEdit := policyEdit{}
	if existing == nil || !slices.Equal(decoded.Managed, managed) {
		managedEdit, err = replaceManaged(current, root, managed)
	}
	if err != nil {
		return nil, err
	}
	return verifyManagedPolicy(applyPolicyEdits(current, []policyEdit{versionEdit, managedEdit}), version, managed)
}

func replaceVersion(data []byte, root *yaml.Node, version string) (policyEdit, error) {
	_, kit := mappingValue(root, "kit")
	_, value := mappingValue(kit, "version")
	if value == nil || value.Kind != yaml.ScalarNode {
		return policyEdit{}, unsupportedManaged("kit.version", "expected a scalar")
	}
	start, end, err := managedSpan(data, value)
	if err != nil {
		return policyEdit{}, err
	}
	if value.Value == version {
		return policyEdit{}, nil
	}
	replacement := yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: version, Style: value.Style}
	encoded, err := yaml.Marshal(&replacement)
	if err != nil {
		return policyEdit{}, fmt.Errorf("bootstrap: encode kit.version: %w", err)
	}
	return policyEdit{start: start, end: end, value: bytes.TrimSuffix(encoded, []byte("\n"))}, nil
}

func replaceManaged(data []byte, root *yaml.Node, managed []policy.ManagedFile) (policyEdit, error) {
	key, value := mappingValue(root, managedKey)
	if value == nil {
		return appendManaged(data, root, managed)
	}
	start, end, err := managedSpan(data, value)
	if err != nil {
		return policyEdit{}, err
	}
	if commentErr := preserveManagedComments(value, data[start:end]); commentErr != nil {
		return policyEdit{}, commentErr
	}
	flow := root.Style&yaml.FlowStyle != 0 || bytes.ContainsAny(data[start:end], "\r\n") && value.Style&yaml.FlowStyle != 0
	encoded, err := encodeManaged(managed, flow, policyNewline(data))
	if err != nil {
		return policyEdit{}, err
	}
	if !flow && key.Line == value.Line {
		start = skipValueSpace(data, start)
		encoded = append([]byte(policyNewline(data)+"  "), encoded...)
	} else if !flow {
		encoded = bytes.ReplaceAll(encoded, []byte("\n  "), []byte("\n"+strings.Repeat(" ", value.Column-1)))
		if len(managed) == 0 && value.Column <= key.Column {
			encoded = append([]byte("  "), encoded...)
		}
	}
	return policyEdit{start: start, end: end, value: encoded}, nil
}

func appendManaged(data []byte, root *yaml.Node, managed []policy.ManagedFile) (policyEdit, error) {
	if root.Style&yaml.FlowStyle != 0 {
		return policyEdit{}, unsupportedManaged(managedKey, "use a block mapping before appending managed")
	}
	for line := range bytes.SplitSeq(data, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("...")) {
			return policyEdit{}, unsupportedManaged(managedKey, "remove the document end marker before appending managed")
		}
	}
	newline := policyNewline(data)
	encoded, err := encodeManaged(managed, false, newline)
	if err != nil {
		return policyEdit{}, err
	}
	prefix := ""
	if len(data) != 0 && data[len(data)-1] != '\n' {
		prefix = newline
	}
	indent := strings.Repeat(" ", root.Column-1)
	text := prefix + indent + "managed:" + newline + indent + "  " + strings.ReplaceAll(string(encoded), newline, newline+indent) + newline
	return policyEdit{start: len(data), end: len(data), value: []byte(text)}, nil
}

func encodeManaged(managed []policy.ManagedFile, flow bool, newline string) ([]byte, error) {
	var node yaml.Node
	if err := node.Encode(managed); err != nil {
		return nil, fmt.Errorf("bootstrap: encode managed: %w", err)
	}
	if flow {
		node.Style = yaml.FlowStyle
	}
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(&node); err != nil {
		return nil, fmt.Errorf("bootstrap: render managed: %w", err)
	}
	encoded := strings.TrimSuffix(output.String(), "\n")
	if !flow {
		encoded = strings.ReplaceAll(encoded, "\n", "\n  ")
	}
	return []byte(strings.ReplaceAll(encoded, "\n", newline)), nil
}

func applyPolicyEdits(data []byte, edits []policyEdit) []byte {
	slices.SortFunc(edits, func(a, b policyEdit) int { return b.start - a.start })
	out := bytes.Clone(data)
	for _, edit := range edits {
		out = slices.Concat(out[:edit.start], edit.value, out[edit.end:])
	}
	return out
}

func policyNewline(data []byte) string {
	if bytes.Contains(data, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

func unsupportedManaged(value, reason string) error {
	return fmt.Errorf("%w: bootstrap: cannot splice %s: %s", model.ErrPolicy, value, reason)
}

func verifyManagedPolicy(data []byte, version string, managed []policy.ManagedFile) ([]byte, error) {
	decoded, err := policy.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: spliced managed policy: %w", err)
	}
	if decoded.Kit.Version != version || !slices.Equal(decoded.Managed, managed) {
		return nil, unsupportedManaged(managedKey, "replacement did not retain the requested values")
	}
	return data, nil
}
