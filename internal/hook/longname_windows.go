//go:build windows

package hook

import "path/filepath"

// longName is an existing directory or file, none of whose elements is a
// link, named as filepath.EvalSymlinks names the root of a repository on
// Windows: each element by its long name and its case on disk, so an 8.3
// short name such as PROGRA~1 is read as the name it stands for. A path
// the system cannot name, as one removed meanwhile, is kept as given.
func longName(path string) string {
	if long, err := filepath.EvalSymlinks(path); err == nil { // fallback: a path the system cannot name keeps its spelling
		return long
	}
	return path
}
