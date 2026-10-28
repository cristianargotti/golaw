// Package integration holds a flow the pipeline excludes from coverage.
package integration

// Flow names the steps of the end to end run.
func Flow() []string { return []string{"build", "serve"} }
