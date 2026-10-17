package source

import (
	"fmt"
	"io/fs"
)

// statIdentity keeps the metadata of the same lstat that established the
// read bound, without asking the path for bytes or another stat.
func statIdentity(info fs.FileInfo) string {
	modified := info.ModTime()
	return fmt.Sprintf("%d:%d:%d%s", info.Size(), modified.Unix(), modified.Nanosecond(), statChange(info))
}
