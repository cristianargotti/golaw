//go:build windows

package runner

import (
	"errors"
	"math"
	"syscall"
)

func treeProcessGone(pid int) bool {
	const queryLimitedInformation, invalidParameter, stillActive = 0x1000, syscall.Errno(87), 259
	if pid <= 0 || uint64(pid) > math.MaxUint32 {
		return false
	}
	handle, err := syscall.OpenProcess(queryLimitedInformation, false, uint32(pid))
	if err != nil {
		return errors.Is(err, invalidParameter)
	}
	var code uint32
	queryErr := syscall.GetExitCodeProcess(handle, &code)
	closeErr := syscall.CloseHandle(handle)
	return queryErr == nil && closeErr == nil && code != stillActive
}
