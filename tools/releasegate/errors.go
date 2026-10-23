package main

import "errors"

var (
	// errUsage marks a command line the gate refuses.
	errUsage = errors.New("usage: releasegate -sha <commit> -runs <file>")
	// errRuns marks a file of runs the gate cannot read or that is not an
	// answer of the runs endpoint.
	errRuns = errors.New("read the runs of ci.yml")
)
