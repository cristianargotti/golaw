//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd

package runner

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
)

func TestTreeExecutableModeWithRestrictiveMask(t *testing.T) {
	t.Parallel()
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), program, "-test.run=^TestTreeExecutableModeChild$")
	cmd.Env = append(gittest.Isolated(), "GOLAW_TREE_MODE_CHILD=1", "GORACE=atexit_sleep_ms=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("executable mode child: %v\n%s", err, output)
	}
}

func TestTreeExecutableModeChild(t *testing.T) {
	t.Parallel()
	if os.Getenv("GOLAW_TREE_MODE_CHILD") != "1" {
		return
	}
	root := payloadRoot(t)
	// Only the isolated child changes umask, leaving concurrent tests untouched.
	previous := syscall.Umask(0o111)
	defer syscall.Umask(previous)
	entry := treeEntry{path: "executable", mode: "100755", size: 3}
	if err := writeTreeBlob(t.Context(), root, strings.NewReader("run"), entry, copyTreeBlob); err != nil {
		t.Fatal(err)
	}
	info, err := root.Stat(entry.path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("executable permissions = %v", info.Mode().Perm())
	}
}
