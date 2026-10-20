package assets

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"path"
	"text/template/parse"

	"gopkg.in/yaml.v3"
)

func parseRendered(name string, data []byte) error {
	var err error
	switch path.Ext(name) {
	case ".yaml", ".yml":
		err = parseRenderedYAML(data)
	case ".json":
		var document any
		err = json.Unmarshal(data, &document)
	case ".go":
		_, err = parser.ParseFile(token.NewFileSet(), name, data, parser.SkipObjectResolution)
	}
	if err != nil {
		return fmt.Errorf("parse rendered %s: %w", name, err)
	}
	return nil
}

func parseRenderedYAML(data []byte) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var document any
		err := decoder.Decode(&document)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("decode rendered YAML: %w", err)
		}
	}
}

func renderChildren(node parse.Node) []parse.Node {
	switch value := node.(type) {
	case *parse.ListNode:
		if value != nil {
			return value.Nodes
		}
	case *parse.PipeNode:
		return pipeChildren(value)
	case *parse.ActionNode:
		return []parse.Node{value.Pipe}
	case *parse.CommandNode:
		return value.Args
	case *parse.IfNode:
		return branchChildren(value.BranchNode)
	case *parse.RangeNode:
		return branchChildren(value.BranchNode)
	case *parse.WithNode:
		return branchChildren(value.BranchNode)
	case *parse.TemplateNode:
		return []parse.Node{value.Pipe}
	case *parse.ChainNode:
		return []parse.Node{value.Node}
	}
	return nil
}

func pipeChildren(pipe *parse.PipeNode) []parse.Node {
	if pipe == nil {
		return nil
	}
	nodes := make([]parse.Node, len(pipe.Cmds))
	for i, command := range pipe.Cmds {
		nodes[i] = command
	}
	return nodes
}

func branchChildren(branch parse.BranchNode) []parse.Node {
	return []parse.Node{branch.Pipe, branch.List, branch.ElseList}
}

func validateReferences(node parse.Node, c Context, scope string) error {
	if err := validateReference(node, c, scope); err != nil {
		return err
	}
	switch value := node.(type) {
	case *parse.WithNode:
		return validateScopedBranch(value.BranchNode, c, scope, withScope(value.Pipe, scope))
	case *parse.RangeNode:
		return validateScopedBranch(value.BranchNode, c, scope, unknownScope)
	}
	for _, child := range renderChildren(node) {
		if err := validateReferences(child, c, scope); err != nil {
			return err
		}
	}
	return nil
}

func validateReference(node parse.Node, c Context, scope string) error {
	if err := validateField(scopedReference(node, scope), c); err != nil {
		return err
	}
	switch value := node.(type) {
	case *parse.CommandNode:
		return validateToolIndex(value, c, scope)
	case *parse.PipeNode:
		return validatePipedIndex(value, c, scope)
	}
	return nil
}

func validateScopedBranch(branch parse.BranchNode, c Context, outer, inner string) error {
	if err := validateReferences(branch.Pipe, c, outer); err != nil {
		return err
	}
	if err := validateReferences(branch.List, c, inner); err != nil {
		return err
	}
	return validateReferences(branch.ElseList, c, outer)
}
