package check

import "context"

// Wait ends a gate a Gate began and answers its results. A gate answers
// one result per name it owns: the test gate also answers coverage, which
// reads the profile the tests wrote.
type Wait func() []Result

// Gate begins one step of the pipeline judged over the change and answers
// the Wait that ends it: a tool gate starts its process and returns, an
// in-process gate does its work and returns what it found.
type Gate func(ctx context.Context) Wait

// Done is the Wait of a gate whose work is already finished.
func Done(results ...Result) Wait { return func() []Result { return results } }

// Run begins every gate before it waits for any, so the tools run at once
// as processes while the in-process gates work, and answers the results in
// the order of the gates, never stopping at the first red. The gates read
// the worktree and write only their own temporary files, so they never
// race, and no goroutine is started that could outlive the run.
func Run(ctx context.Context, gates []Gate) []Result {
	waits := make([]Wait, 0, len(gates))
	for _, gate := range gates {
		waits = append(waits, gate(ctx))
	}
	results := make([]Result, 0, len(waits))
	for _, wait := range waits {
		results = append(results, wait()...)
	}
	return results
}
