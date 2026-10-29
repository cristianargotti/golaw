package lint

import (
	"embed"
	"io/fs"
)

// The file names the Python drivers hand to their tools; each set
// directory of the tree holds one of each.
const (
	pythonRuffConfig    = "ruff.toml"
	pythonPyrightConfig = "pyrightconfig.json"
	pythonConfigRoot    = "config/python"
)

// pythonConfigFiles is embedded so the configuration travels with the
// binary and is materialized into the tool directory, never the checkout.
//
// The parity pyright runs off with only the rule of python:S8495 at error,
// because parity holds the mapped rules alone. Both sets turn the missing
// import reports off: a kit-owned configuration cannot name the
// repository's import roots, and an unresolved import of the repository's
// own module is not a defect of the file.
//
//go:embed config/python
var pythonConfigFiles embed.FS

// pythonConfig answers the Python tree with the set directories at its
// root, the shape a driver's Config carries.
func pythonConfig() fs.FS {
	tree, err := fs.Sub(pythonConfigFiles, pythonConfigRoot)
	if err != nil {
		// fs.Sub fails only on an invalid path, and the root is a constant.
		return pythonConfigFiles
	}
	return tree
}
