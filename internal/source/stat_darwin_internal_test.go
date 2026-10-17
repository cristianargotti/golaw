//go:build darwin

package source

import (
	"io/fs"
	"os"
	"syscall"
	"testing"
)

// TestStatIdentityKeepsChangeTimeAndInode distinguishes each change time
// field and the inode while the size and modification time stay fixed.
func TestStatIdentityKeepsChangeTimeAndInode(t *testing.T) {
	t.Parallel()
	info, err := os.Lstat(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	before := stampedFileInfo{FileInfo: info, stat: syscall.Stat_t{
		Ino: 17, Ctimespec: syscall.Timespec{Sec: 23, Nsec: 41},
	}}
	for _, tc := range []struct {
		name   string
		change func(*syscall.Stat_t)
	}{
		{"change time seconds", func(stat *syscall.Stat_t) { stat.Ctimespec.Sec++ }},
		{"change time nanoseconds", func(stat *syscall.Stat_t) { stat.Ctimespec.Nsec++ }},
		{"inode", func(stat *syscall.Stat_t) { stat.Ino++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			after := before
			tc.change(&after.stat)
			if statIdentity(after) == statIdentity(before) {
				t.Fatalf("changing only %s must change the stat identity", tc.name)
			}
		})
	}
}

// stampedFileInfo holds fixed portable metadata with a synthetic Unix stat.
type stampedFileInfo struct {
	fs.FileInfo

	stat syscall.Stat_t
}

// Sys answers the synthetic stat while every portable field stays fixed.
func (info stampedFileInfo) Sys() any { return &info.stat }
