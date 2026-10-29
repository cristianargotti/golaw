package lint

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/surface"
)

// The pinned tools of the web surface.
const (
	eslintTool    = "eslint"
	stylelintTool = "stylelint"
)

// eslintDriver runs ESLint over scripts and the scripts inside HTML. It has
// no hook invocation: a typed run is too slow for the hook (M6.6.1).
func eslintDriver() Driver {
	return Driver{
		Tool:      eslintTool,
		Languages: []surface.Language{surface.JavaScript, surface.TypeScript, surface.JSX, surface.TSX, surface.HTML},
		Format:    FormatESLint,
		Args:      eslintArgs,
		Config:    webConfig(),
	}
}

// stylelintDriver runs stylelint over CSS and the style blocks of HTML,
// in the lint target only (M6.6.2).
func stylelintDriver() Driver {
	return Driver{
		Tool:      stylelintTool,
		Languages: []surface.Language{surface.CSS, surface.HTML},
		Format:    FormatStylelint,
		Args:      stylelintArgs,
		Config:    webConfig(),
	}
}

// eslintArgs leaves out --no-config-lookup: --config already makes flat
// ESLint read that file alone, and it never reads an .eslintrc file.
func eslintArgs(in Input) []string {
	args := []string{
		configFlag, filepath.Join(in.ConfigDir, eslintConfigFile),
		"--format", "json", "--no-warn-ignored", "--no-error-on-unmatched-pattern",
	}
	if len(in.Globals) > 0 {
		args = append(args, "--global", strings.Join(in.Globals, ","))
	}
	return append(args, in.Files...)
}

// stylelintArgs names the kit's empty ignore file, so a repository
// .stylelintignore cannot hide a file of the analyzer scope.
func stylelintArgs(in Input) []string {
	head := []string{
		configFlag, filepath.Join(in.ConfigDir, stylelintConfigFile),
		"--ignore-path", filepath.Join(in.ConfigDir, stylelintIgnoreFile),
		"--formatter", "json", "--allow-empty-input",
	}
	return slices.Concat(head, in.Files)
}

// webDrivers answers the web drivers in tool order.
func webDrivers() []Driver { return []Driver{eslintDriver(), stylelintDriver()} }
