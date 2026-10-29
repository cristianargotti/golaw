package lint

import (
	"embed"
	"io/fs"
)

// webEmbedded holds the kit's own web configurations, so a repository file
// never weakens a set nor runs code at lint time (M6.4.1).
//
//go:embed config/web
var webEmbedded embed.FS

// The web configuration tree and the names of its files in each set.
const (
	webConfigRoot       = "config/web"
	eslintConfigFile    = "eslint.config.mjs"
	stylelintConfigFile = "stylelint.config.mjs"
	stylelintIgnoreFile = "golaw.stylelintignore"
	webRulesFile        = "web-rules.json"
	tsconfigFile        = "tsconfig.golaw.json"
)

// webConfig answers the tree with parity/ and strict/ at its root.
func webConfig() fs.FS {
	tree, err := fs.Sub(webEmbedded, webConfigRoot)
	if err != nil {
		// fs.Sub fails only on an invalid path, and the root is a constant.
		return webEmbedded
	}
	return tree
}
