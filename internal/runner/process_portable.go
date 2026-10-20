//go:build !darwin && !dragonfly && !freebsd && !illumos && !linux && !netbsd && !openbsd && !windows

package runner

func treeProcessGone(_ int) bool { return false }
