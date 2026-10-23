//go:build !windows

package hook

// longName is the path itself where the system names a file by one name
// alone: an existing path whose links the walk followed is already named
// as filepath.EvalSymlinks names the root.
func longName(path string) string { return path }
