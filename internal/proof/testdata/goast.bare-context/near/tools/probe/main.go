package main

import "context"

// A main package outside cmd is a composition root: it creates the context
// it hands to the packages below it.
func main() {
	_ = context.Background()
}
