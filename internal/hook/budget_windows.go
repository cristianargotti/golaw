//go:build windows

package hook

import "time"

// internalBudget leaves the host four of its ten seconds on Windows, where
// the installed command reaches the binary through a PowerShell launcher:
// the first native run spent about a second and a half per installed call,
// so a slow path held to eight seconds answered past the acceptance bound,
// near the point where a host with no fail-closed setting runs the tool.
const internalBudget = 6 * time.Second
