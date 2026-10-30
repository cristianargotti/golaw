//go:build !windows

package main

// shortPath is the path itself: only Windows starts a shim through cmd.exe.
func shortPath(path string) string { return path }
