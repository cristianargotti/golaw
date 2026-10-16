package domain

import "time"

// Expired reads the process clock inside the domain.
func Expired(until time.Time) bool {
	return time.Now().After(until)
}
