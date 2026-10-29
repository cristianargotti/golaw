package lint

import (
	"embed"
	"io/fs"
)

// The configuration files of the infrastructure tools, at the root of a set.
const (
	cfnLintConfig  = ".cfnlintrc.yaml"
	checkovConfig  = "checkov.yaml"
	hadolintConfig = ".hadolint.yaml"
	yamllintConfig = ".yamllint.yaml"
)

// infraConfigRoot is the directory of the tree inside the embed.
const infraConfigRoot = "config/infra"

// infraTree holds both sets; all: keeps the dotted names the tools expect.
//
//go:embed all:config/infra
var infraTree embed.FS

// infraConfig answers the tree with the set directories parity/ and strict/
// at its root, the configuration every infrastructure driver carries.
func infraConfig() fs.FS {
	sub, err := fs.Sub(infraTree, infraConfigRoot)
	if err != nil {
		// fs.Sub fails only on an invalid name, and the name is a constant.
		panic(err)
	}
	return sub
}
