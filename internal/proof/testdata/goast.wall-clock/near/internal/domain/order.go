package domain

import "time"

// Expired takes now as a parameter.
func Expired(now, until time.Time) bool {
	return now.After(until)
}
