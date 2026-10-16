package main

import "time"

// Clock adapts a clock function for the wiring.
func Clock(now func() time.Time) func() time.Time {
	return now
}

func main() {
	clock := Clock(time.Now)
	_ = clock
}
