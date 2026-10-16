package main

import "time"

// Clock adapts a clock function for the wiring.
func Clock(now func() time.Time) func() time.Time {
	return now
}

func utcNow() time.Time {
	return time.Now().UTC()
}

func main() {
	clock := Clock(utcNow)
	_ = clock
}
