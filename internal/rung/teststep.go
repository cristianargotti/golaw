package rung

import (
	"bytes"
	"context"
	"fmt"

	"github.com/cristianargotti/golaw/internal/runner"
)

// testStep names the go test step test_staged arms, and raceNeedsCgo is
// what go prints when -race meets a host whose cgo is off.
const (
	testStep     = "test"
	raceNeedsCgo = "-race requires cgo"
)

// test runs go test -race over the affected packages when the ladder arms
// test_staged (decision 8: off by default, because the suite of a real
// commit took minutes; the full suite stays in make ci). The tests run with
// the user environment, so a test that spawns git answers for its own
// directory and never for the repository the hook runs in.
func (p *precommit) test(ctx context.Context, t goTree) Outcome {
	switch {
	case !p.ladder.TestStaged:
		return passed(testStep, "skipped, test_staged is off in the ladder of golaw.yml")
	case len(t.set.Patterns) == 0:
		return passed(testStep, noPackage)
	}
	out, err := runner.Run(ctx, p.goCommand(t, "test", "-race"))
	switch {
	case err != nil:
		return stepFailed(testStep, err)
	case out.Code == 0:
		return passed(testStep, fmt.Sprintf("go test -race passes in %d affected packages", len(t.set.Packages)))
	case bytes.Contains(out.Stderr, []byte(raceNeedsCgo)):
		return passed(testStep, "go test -race needs cgo and a C compiler this host lacks: install a C compiler or switch test_staged off in golaw.yml; the pipeline runs the tests")
	default:
		return refusedBy(testStep, "go test -race fails in the affected packages; fix the tests before the commit", out)
	}
}
