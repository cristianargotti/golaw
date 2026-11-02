package goldentest

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
)

// The commits of a run: the base is committed by start, the generated
// files and the fills each get their own, so coverage on new code and the
// duplication measure read the change a pipeline would read.
const (
	generatedMessage = "feat: generate the slice"
	filledMessage    = "feat: fill the holes"
	sliceWhere       = "slice"
)

// The suffixes and the message the steps share.
const (
	goSuffix   = ".go"
	testSuffix = "_test.go"
	readFailed = "read %s: %w"
)

// Run proves a golden slice and fails t with every failure; each skipped
// step becomes a skipped subtest carrying its reason, so go test -v and
// -json list what was not proven instead of passing it silently.
func Run(t *testing.T, s Slice, steps ...Step) Report {
	t.Helper()
	report, failures := Exec(t, s, steps...)
	for _, step := range order() {
		if reason, skipped := report.Skipped[step]; skipped {
			t.Run(string(step)+" skipped", func(t *testing.T) { t.Skip(reason) })
		}
	}
	for _, failure := range failures {
		t.Errorf("%s: %s", s.Name, failure)
	}
	return report
}

// Exec runs a golden slice and answers its failures instead of failing the
// test, so the red slices of the harness's own tests are provable. No step
// named means every step.
func Exec(tb testing.TB, s Slice, steps ...Step) (Report, []string) {
	tb.Helper()
	chosen, err := selected(steps)
	if err == nil {
		err = validate(s)
	}
	if err != nil {
		return Report{}, []string{sliceWhere + ": " + err.Error()}
	}
	ctx := tb.Context()
	h, err := start(tb, s)
	if err != nil {
		return Report{}, []string{sliceWhere + ": " + err.Error()}
	}
	if !h.generate(ctx) {
		return h.report, h.failures
	}
	if chosen[StepCheck] {
		h.checkOpen(ctx)
	}
	h.commit(generatedMessage)
	if chosen[StepCoverage] {
		h.openCoverage(ctx)
	}
	if !h.fill(ctx) {
		return h.report, h.failures
	}
	h.commit(filledMessage)
	for _, step := range order() {
		if chosen[step] {
			h.run(ctx, step)
		}
	}
	return h.report, h.failures
}

// selected answers the steps a run proves; none named means all of them.
func selected(steps []Step) (map[Step]bool, error) {
	known := order()
	if len(steps) == 0 {
		steps = known
	}
	chosen := make(map[Step]bool, len(steps))
	for _, step := range steps {
		if !slices.Contains(known, step) {
			return nil, fmt.Errorf("%w: unknown step %s", ErrSlice, step)
		}
		chosen[step] = true
	}
	return chosen, nil
}

// validate refuses a slice that could prove nothing.
func validate(s Slice) error {
	switch {
	case s.Module == "":
		return fmt.Errorf("%w: no module", ErrSlice)
	case len(s.Base["go.mod"]) == 0:
		return fmt.Errorf("%w: the base holds no go.mod", ErrSlice)
	case len(s.Base["golaw.yml"]) == 0:
		return fmt.Errorf("%w: the base holds no golaw.yml", ErrSlice)
	case len(s.Kinds) == 0:
		return fmt.Errorf("%w: no kind", ErrSlice)
	case len(s.Commands) == 0:
		return fmt.Errorf("%w: no command", ErrSlice)
	}
	return nil
}

// harness is the state of one run. It holds no context: every method takes
// the test's own, so a run ends when the test does.
type harness struct {
	tb       testing.TB
	s        Slice
	repo     *gittest.Repo
	base     string // the base commit
	policy   *model.EffectivePolicy
	manifest manifest.Manifest
	work     string // profiles and the lint configuration, outside the module so no scan reads them
	profile  string // the pipeline profile once written
	report   Report
	failures []string
	// resolveLint answers golangci-lint and why the lint steps skip; nil
	// resolves the pinned tool, a test passes a fake.
	resolveLint func(ctx context.Context, root string) (string, string)
	// surfaceSet is the set the surface step judges under; empty is the
	// strict set init writes, a test passes another.
	surfaceSet string
	// floorBinary answers the go command of the floor step and why it
	// skips; nil resolves the floor release, a test passes a fake.
	floorBinary func(ctx context.Context, work string) (string, string)
}

// failf records one failure; where is a step, the slice, a command or a fill.
func (h *harness) failf(where, format string, args ...any) {
	h.failures = append(h.failures, where+": "+fmt.Sprintf(format, args...))
}

// skip records a step left unproven and why.
func (h *harness) skip(step Step, reason string) {
	h.report.Skipped[step] = reason
}

// commit records everything the run wrote as one commit.
func (h *harness) commit(message string) {
	h.repo.Git("add", "-A")
	h.repo.Commit(message, nil)
}

// run proves one step.
func (h *harness) run(ctx context.Context, step Step) {
	switch step {
	case StepBuild:
		h.build(ctx)
	case StepLint:
		h.lint(ctx)
	case StepScan:
		h.scan(ctx)
	case StepSurface:
		h.surface(ctx)
	case StepCoverage:
		h.coverage(ctx)
	case StepDup:
		h.duplication(ctx)
	case StepCheck:
		h.checkClean(ctx)
	case StepFormat:
		h.format(ctx)
	case StepLaws:
		h.laws()
	case StepFloor:
		h.floor(ctx)
	}
}

// laws judges the files as the commands wrote them, before the fills,
// because the laws are about what a template emits.
func (h *harness) laws() {
	for _, law := range Laws() {
		for _, message := range law.Assert(h.report.Files) {
			h.failf(string(StepLaws), "%s", message)
		}
	}
}
