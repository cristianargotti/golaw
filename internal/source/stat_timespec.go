//go:build darwin || freebsd || netbsd

package source

import (
	"fmt"
	"io/fs"
	"syscall"
)

// statChange appends the change time and inode from the same lstat.
func statChange(info fs.FileInfo) string {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	return fmt.Sprintf(":%d:%d:%d", stat.Ctimespec.Sec, stat.Ctimespec.Nsec, stat.Ino)
}
