package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cristianargotti/golaw/internal/check"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
)

// genSummaryPattern matches the count line golaw gen --check ends with,
// which the gate's own count line already carries.
var genSummaryPattern = regexp.MustCompile(`^\d+ drift, \d+ open holes$`)

// testGate runs go test over the affected packages with a coverage
// profile and then measures the new code from it, as make coverage-new
// does; coverage answers apart so a red test and a red measure read apart.
// The event stream goes to a file, so a suite of any size keeps its end.
func (p *checkPlan) testGate(ctx context.Context) check.Wait {
	if reason := p.goReason(); reason != "" {
		return check.Done(check.Skip(gateTest, reason), check.Skip(gateCoverage, reason))
	}
	if p.tools.golang == "" {
		return check.Done(check.Skip(gateTest, missing("go")), check.Skip(gateCoverage, missing("go")))
	}
	profile, events := filepath.Join(p.temp, "profile.cov"), filepath.Join(p.temp, "test.json")
	stream, err := os.Create(events) //nolint:gosec // a file of the run's own temporary directory
	if err != nil {
		return check.Done(testNotRun(fmt.Errorf("the event file: %w", err))...)
	}
	args := append([]string{"test", "-json", "-coverprofile=" + profile}, p.affected.Patterns...)
	process, err := runner.Start(ctx, runner.Command{Name: p.tools.golang, Args: args, Dir: p.root, Env: p.env}, stream)
	if err != nil {
		_ = stream.Close() // the start failure is the result the person reads
		return check.Done(testNotRun(err)...)
	}
	return func() []check.Result {
		out, err := process.Wait(ctx)
		if err = errors.Join(err, stream.Close()); err != nil {
			return testNotRun(err)
		}
		failures, err := p.testFailures(events)
		if err != nil {
			return testNotRun(err)
		}
		if out.Code != 0 && len(failures) == 0 {
			failures = check.Failed("go test", out.Code, nil, out.Stderr)
		}
		if len(failures) > 0 {
			return []check.Result{check.Fail(gateTest, failures...), check.Skip(gateCoverage, "the tests are red")}
		}
		return []check.Result{check.Pass(gateTest), p.coverage(ctx, profile)}
	}
}

func testNotRun(err error) []check.Result {
	return []check.Result{check.Fail(gateTest, "could not run go test: "+err.Error()), check.Skip(gateCoverage, "the tests did not run")}
}

// testFailures reads the event file go test wrote.
func (p *checkPlan) testFailures(events string) ([]string, error) {
	file, err := os.Open(events) //nolint:gosec // a file of the run's own temporary directory
	if err != nil {
		return nil, fmt.Errorf("open the events: %w", err)
	}
	failures, err := check.TestFailures(file, p.packageDir)
	if err = errors.Join(err, file.Close()); err != nil {
		return nil, fmt.Errorf("read the events: %w", err)
	}
	return failures, nil
}

// packageDir names a package of the module by its directory from the root.
func (p *checkPlan) packageDir(pkg string) string {
	if pkg == p.module {
		return ""
	}
	return strings.TrimPrefix(pkg, p.module+"/")
}

// coverage measures the new lines of the worktree through the code path of
// golaw coverage --new-code, over the profile the tests just wrote.
func (p *checkPlan) coverage(ctx context.Context, profile string) check.Result {
	if p.scope.Whole {
		return check.Skip(gateCoverage, "no base names the new code")
	}
	b := p.a.bind()
	options := coverageOptions{Base: p.scope.Base, Input: profile, NewCode: true, Worktree: true}
	report, _, err := b.coverageNew(ctx, p.root, p.a.now(), options)
	if off := (coverageOffError{}); errors.As(err, &off) {
		return check.Skip(gateCoverage, "the go capability is off")
	}
	switch {
	case err != nil:
		return check.Fail(gateCoverage, err.Error())
	case report.Status.Kind == model.StatusUnmeasured:
		return check.Fail(gateCoverage, "unmeasured: "+report.Status.Reason)
	case report.Status.Kind == model.StatusMeasured && !report.Passed():
		return check.Fail(gateCoverage, report.Text()...)
	}
	return check.Pass(gateCoverage)
}

// scanGate runs the golaw rules over the worktree, as golaw scan . does,
// and keeps the findings of the changed files and those of no file.
func (p *checkPlan) scanGate(ctx context.Context) check.Wait {
	run, err := p.dispatch(ctx)
	if err != nil {
		return check.Done(check.Fail(gateScan, err.Error()))
	}
	decision := engine.Decide(run)
	var lines []string
	for _, unmeasured := range decision.Unmeasured {
		lines = append(lines, "unmeasured "+unmeasured.Check+": "+unmeasured.Reason)
	}
	for i := range decision.Findings {
		f := &decision.Findings[i]
		if f.Enforced() && (f.Path == "" || p.scope.Changed(f.Path)) {
			lines = append(lines, findingLine(*f))
		}
	}
	if len(lines) > 0 {
		return check.Done(check.Fail(gateScan, lines...))
	}
	return check.Done(check.Pass(gateScan))
}

func (p *checkPlan) dispatch(ctx context.Context) (*engine.Run, error) {
	snapshot, err := p.a.snapshot(ctx, p.root)
	if err != nil {
		return nil, scanFailed(err)
	}
	registry, err := p.a.registry()
	if err != nil {
		return nil, scanFailed(err)
	}
	run, err := engine.Dispatch(ctx, registry, model.Input{Mode: model.ModeScan, Policy: p.policy, Snapshot: snapshot, Targets: snapshot.Files()})
	if err != nil {
		return nil, scanFailed(err)
	}
	return run, nil
}

// findingLine prints a finding as file:line check: message.
func findingLine(f model.Finding) string {
	where := f.Path
	if f.Line > 0 {
		where = fmt.Sprintf("%s:%d", f.Path, f.Line)
	}
	if where == "" {
		return f.Check + ": " + f.Message
	}
	return where + " " + f.Check + ": " + f.Message
}

// generateGate runs golaw gen --check when the repository generates code:
// a generate block in golaw.yml or a generation manifest.
func (p *checkPlan) generateGate(ctx context.Context) check.Wait {
	if !p.generates() {
		return check.Done(check.Skip(gateGenerate, "the repository declares no generation"))
	}
	if !p.scope.Whole && len(p.scope.Paths) == 0 {
		return check.Done(check.Skip(gateGenerate, "nothing changed"))
	}
	return check.Done(p.generated(ctx))
}

// generated runs golaw gen --check in process and keeps its finding lines.
func (p *checkPlan) generated(ctx context.Context) check.Result {
	var out bytes.Buffer
	err := p.a.checkGen(ctx, &out, io.Discard, p.root, genOptions{check: true})
	if exit := (exitError{}); err != nil && !errors.As(err, &exit) {
		return check.Fail(gateGenerate, err.Error())
	}
	if err == nil {
		return check.Pass(gateGenerate)
	}
	var lines []string
	for line := range strings.SplitSeq(strings.TrimSpace(out.String()), "\n") {
		line = strings.TrimPrefix(line, checkPrefix)
		if line != "" && !genSummaryPattern.MatchString(line) {
			lines = append(lines, line)
		}
	}
	return check.Fail(gateGenerate, lines...)
}

func (p *checkPlan) generates() bool {
	if p.policy.Generate.Declared {
		return true
	}
	_, err := os.Stat(filepath.Join(p.root, filepath.FromSlash(manifest.Path)))
	return err == nil
}
