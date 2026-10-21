package policyfiles

import (
	"github.com/BurntSushi/toml"
	"golang.org/x/mod/modfile"
)

// decodeTOML decodes a TOML configuration whole into v: the gitleaks and the
// squawk files are TOML, and a subset reader would turn a valid file (a
// multi-line literal regular expression is ordinary in gitleaks) into a
// stated limit that fails the pipeline. A syntax error names the file and
// the line the decoder reports.
func decodeTOML(path string, data []byte, v any) error {
	if _, err := toml.Decode(string(data), v); err != nil {
		return parseError(path, err)
	}
	return nil
}

// parseModule parses a go.mod the way the go command does, every require
// and replace block included, so the dependency checks read the module
// graph the toolchain reads and never a text match of it.
func parseModule(path string, data []byte) (*modfile.File, error) {
	f, err := modfile.Parse(path, data, nil)
	if err != nil {
		return nil, parseError(path, err)
	}
	return f, nil
}

// modulePath answers the module path a go.mod declares, empty when it
// declares none.
func modulePath(data []byte) string {
	return modfile.ModulePath(data)
}
