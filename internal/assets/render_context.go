package assets

import (
	"fmt"
	"strings"
	"text/template/parse"
)

const (
	toolsScope   = "Tools"
	unknownScope = "unknown"
)

// Requirement preserves the adopter's dependency choices for generated records.
type Requirement struct {
	Path    string
	Version string
}

// Context adds optional rendering inputs without changing the fixed Values API.
type Context struct {
	Values

	CIBackend      string
	DirectRequires []Requirement
}

func referenceName(node parse.Node) string {
	switch value := node.(type) {
	case *parse.DotNode:
		return "."
	case *parse.FieldNode:
		return strings.Join(value.Ident, ".")
	case *parse.VariableNode:
		if len(value.Ident) > 0 && value.Ident[0] == "$" {
			return strings.Join(value.Ident, ".")
		}
	case *parse.PipeNode:
		if value != nil && len(value.Cmds) == 1 {
			return referenceName(value.Cmds[0])
		}
	case *parse.CommandNode:
		if len(value.Args) == 1 {
			return referenceName(value.Args[0])
		}
	case *parse.ChainNode:
		return referenceName(value.Node) + "." + strings.Join(value.Field, ".")
	}
	return ""
}

func scopedReference(node parse.Node, scope string) string {
	name := referenceName(node)
	if root, ok := strings.CutPrefix(name, "$."); ok {
		return strings.TrimPrefix(root, "Values.")
	}
	if name == "" || name == "." || name == "$" || scope == unknownScope {
		return ""
	}
	if scope == toolsScope {
		return "Tools." + name
	}
	return strings.TrimPrefix(name, "Values.")
}

func withScope(pipe *parse.PipeNode, outer string) string {
	switch referenceName(pipe) {
	case ".":
		return outer
	case "$":
		return ""
	}
	switch scopedReference(pipe, outer) {
	case "Values":
		return ""
	case toolsScope:
		return toolsScope
	}
	return unknownScope
}

func validateField(name string, c Context) error {
	values := map[string]string{
		"ModulePath": c.ModulePath, "GoVersion": c.GoVersion, "KitVersion": c.KitVersion,
		"Language": c.Language, "Decisions": c.Decisions, "Main": c.Main, "CIBackend": c.CIBackend,
	}
	if value, ok := values[name]; ok && value == "" {
		return fmt.Errorf("%w: required input %s is empty", ErrRender, name)
	}
	if key, ok := strings.CutPrefix(name, "Tools."); ok {
		return validateToolKey(c.Tools, key)
	}
	return nil
}

func validateToolIndex(command *parse.CommandNode, c Context, scope string) error {
	const indexArguments = 3
	if len(command.Args) != indexArguments {
		return nil
	}
	function, ok := command.Args[0].(*parse.IdentifierNode)
	if !ok || function.Ident != "index" || scopedReference(command.Args[1], scope) != toolsScope {
		return nil
	}
	key, ok := command.Args[2].(*parse.StringNode)
	if !ok {
		return nil
	}
	return validateToolKey(c.Tools, key.Text)
}

func validatePipedIndex(pipe *parse.PipeNode, c Context, scope string) error {
	if pipe == nil || len(pipe.Cmds) != 2 || len(pipe.Cmds[0].Args) != 1 || len(pipe.Cmds[1].Args) != 2 {
		return nil
	}
	if _, ok := pipe.Cmds[0].Args[0].(*parse.StringNode); !ok {
		return nil
	}
	command := *pipe.Cmds[1]
	command.Args = append(append([]parse.Node(nil), command.Args...), pipe.Cmds[0].Args[0])
	return validateToolIndex(&command, c, scope)
}

func validateToolKey(tools map[string]string, key string) error {
	if tools[key] == "" {
		return fmt.Errorf("%w: required tool key %q is missing or empty", ErrRender, key)
	}
	return nil
}
