package main

import "os"

// A main package outside cmd is a composition root: it may read the
// environment it hands to the packages below it.
func main() {
	_ = os.Getenv("HOME")
}
