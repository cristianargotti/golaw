//go:build !windows

package install

import "testing"

func TestWindowsLauncherStreamsAndStatus(t *testing.T) {
	t.Parallel()
	cinHome(t)
	t.Skip("requires Windows cmd.exe and Windows PowerShell; no Windows process was exercised")
}
