//go:build !windows

package hook

import "time"

// internalBudget is the deadline of a whole invocation, two seconds below
// the host timeout, where the installed command starts the binary directly.
const internalBudget = 8 * time.Second
