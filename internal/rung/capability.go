package rung

// goCapability is the capability the Go steps of the rungs run under, and
// goOff the line of each Go step while it is off: the policy says the
// repository holds no Go program, so no Go tool is verified or run.
const (
	goCapability = "go"
	goOff        = "skipped, the go capability is off"
)

// goSkipped names every Go step of the pre-commit rung while the go
// capability is off, the test step by the ladder when test_staged is off,
// because a silent step could not be told from one that never ran.
func goSkipped(testStaged bool) Outcome {
	out := Outcome{}
	for _, step := range []string{fmtStep, lintStep, vetStep} {
		out = join(out, passed(step, goOff))
	}
	if !testStaged {
		return join(out, passed(testStep, testStagedOff))
	}
	return join(out, passed(testStep, goOff))
}
