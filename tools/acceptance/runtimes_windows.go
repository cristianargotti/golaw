package main

import "syscall"

// linkProgram writes link.cmd, a shim that starts target: a symbolic link
// needs a privilege a runner may lack, and python.exe started through one
// may miss the libraries beside its real file.
func linkProgram(target, link string) error { return writeShim(link, target) }

// shortPath is the 8.3 name Windows keeps for an existing path: no space
// and no rune outside ASCII, so cmd.exe reads it whole in any code page.
// A volume without short names answers the path as it is.
func shortPath(path string) string {
	long, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return path
	}
	var buf [syscall.MAX_LONG_PATH]uint16
	n, err := syscall.GetShortPathName(long, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || int(n) > len(buf) {
		return path
	}
	return syscall.UTF16ToString(buf[:n])
}
