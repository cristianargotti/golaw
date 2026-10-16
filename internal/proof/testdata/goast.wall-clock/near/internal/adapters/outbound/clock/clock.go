package clock

import "time"

// Now is the process clock, read at the edge.
func Now() time.Time {
	return time.Now()
}
