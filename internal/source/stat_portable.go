//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package source

import "io/fs"

// statChange leaves the stamp at size and modification time where the
// platform exposes no portable change time or inode.
func statChange(fs.FileInfo) string { return "" }
