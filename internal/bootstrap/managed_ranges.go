package bootstrap

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const managedValueName = "value"

func mappingValue(node *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	if node != nil && node.Kind == yaml.MappingNode {
		for index := 0; index+1 < len(node.Content); index += 2 {
			if node.Content[index].Value == key {
				return node.Content[index], node.Content[index+1]
			}
		}
	}
	return nil, nil
}

func managedSpan(data []byte, node *yaml.Node) (int, int, error) {
	if err := supportedManagedNode(node); err != nil {
		return 0, 0, err
	}
	start, err := nodeOffset(data, node)
	if err != nil {
		return 0, 0, err
	}
	if node.Kind == yaml.ScalarNode {
		end, scalarErr := scalarEnd(data, start, node)
		return start, end, scalarErr
	}
	if node.Style&yaml.FlowStyle != 0 {
		end, flowErr := flowEnd(data, start)
		return start, end, flowErr
	}
	if len(node.Content) == 0 {
		return 0, 0, unsupportedManaged(managedValueName, "empty block collection")
	}
	_, end, err := managedSpan(data, node.Content[len(node.Content)-1])
	return start, end, err
}

func supportedManagedNode(node *yaml.Node) error {
	if node.Anchor != "" || node.Kind == yaml.AliasNode || node.Style&(yaml.TaggedStyle|yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return unsupportedManaged(managedValueName, "use ordinary scalars and sequences instead of anchors, aliases, tags or block scalars")
	}
	for _, child := range node.Content {
		if err := supportedManagedNode(child); err != nil {
			return err
		}
	}
	return nil
}

func nodeOffset(data []byte, node *yaml.Node) (int, error) {
	line, column := 1, 1
	for offset := 0; offset < len(data); {
		if line == node.Line && column == node.Column {
			return offset, nil
		}
		runeValue, size := utf8.DecodeRune(data[offset:])
		offset += size
		column++
		if runeValue == '\n' {
			line, column = line+1, 1
		}
	}
	return 0, unsupportedManaged(managedValueName, "source position is outside the document")
}

func scalarEnd(data []byte, start int, node *yaml.Node) (int, error) {
	if node.Style&(yaml.SingleQuotedStyle|yaml.DoubleQuotedStyle) != 0 {
		return quotedEnd(data, start)
	}
	end := start + len(node.Value)
	if end > len(data) || !bytes.Equal(data[start:end], []byte(node.Value)) || bytes.ContainsAny(data[start:end], "\r\n") {
		return 0, unsupportedManaged("scalar", "multiline or implicit values are unsupported")
	}
	return end, nil
}

func quotedEnd(data []byte, start int) (int, error) {
	quote := data[start]
	for index := start + 1; index < len(data); index++ {
		if data[index] == '\n' || data[index] == '\r' {
			break
		}
		if quote == '"' && data[index] == '\\' {
			index++
		} else if data[index] == quote {
			if quote == '\'' && index+1 < len(data) && data[index+1] == quote {
				index++
				continue
			}
			return index + 1, nil
		}
	}
	return 0, unsupportedManaged("scalar", "unterminated or multiline quoted value")
}

func flowEnd(data []byte, start int) (int, error) {
	depth := 0
	for index := start; index < len(data); index++ {
		switch data[index] {
		case '\'', '"':
			end, err := quotedEnd(data, index)
			if err != nil {
				return 0, err
			}
			index = end - 1
		case '#':
			index = skipFlowComment(data, index)
		case '[', '{':
			depth++
		case ']', '}':
			depth--
			if depth == 0 {
				return index + 1, nil
			}
		default:
		}
	}
	return 0, unsupportedManaged("sequence", "unterminated flow collection")
}

func skipValueSpace(data []byte, start int) int {
	for start > 0 && (data[start-1] == ' ' || data[start-1] == '\t') {
		start--
	}
	return start
}

func skipFlowComment(data []byte, index int) int {
	if index == 0 || !strings.ContainsRune(" \t\r\n", rune(data[index-1])) {
		return index
	}
	if offset := bytes.IndexByte(data[index:], '\n'); offset >= 0 {
		return index + offset
	}
	return len(data)
}

func preserveManagedComments(node *yaml.Node, span []byte) error {
	for _, comment := range []string{node.HeadComment, node.LineComment, node.FootComment} {
		if comment != "" && bytes.Contains(span, []byte(comment)) {
			return unsupportedManaged(managedKey, "move comments inside the replaced value above managed before changing its entries")
		}
	}
	for _, child := range node.Content {
		if err := preserveManagedComments(child, span); err != nil {
			return err
		}
	}
	return nil
}
