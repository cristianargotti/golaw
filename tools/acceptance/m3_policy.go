package main

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	m3PolicyCategory = "m3-policy"
	m3Baseline       = "baseline"
	m3ProfileChecks  = "profile-checks"
	m3ServiceCase    = "service"
	// m3TailLines bounds the output a failed make leaves in a row's detail,
	// enough to name the failing target and the finding before it.
	m3TailLines = 12
)

// m3PolicyNames is the baseline, one row per violation in name order and
// the profile row.
func m3PolicyNames() []string {
	violations := m3Violations()
	names := make([]string, 0, len(violations)+2)
	names = append(names, m3Baseline)
	names = append(names, slices.Sorted(maps.Keys(violations))...)
	return append(names, m3ProfileChecks)
}

func (s *suite) policyM3(ctx context.Context, name string) error {
	switch name {
	case m3Baseline:
		return s.baselineM3(ctx)
	case m3ProfileChecks:
		return s.profileM3(ctx)
	}
	violation, found := m3Violations()[name]
	if !found {
		return fmt.Errorf("unknown M3 policy case %q: %w", name, ErrAcceptance)
	}
	return s.m3ViolationRow(ctx, name, violation)
}

// baselineM3 proves the rendered service passes its own make ci before any
// edit, so a violation row fails on its edit alone.
func (s *suite) baselineM3(ctx context.Context) error {
	repo, err := s.m3ServiceRepository(ctx, m3ServiceCase, []string{"go"})
	if err != nil {
		return err
	}
	return s.m3PassesCI(ctx, repo)
}

// profileM3 proves D11 on the rendered service: without a profile the two
// profile checks answer not applicable with a reason that names the
// profile, never a finding and never a measurement.
func (s *suite) profileM3(ctx context.Context) error {
	repo, err := s.m3ServiceRepository(ctx, m3ServiceCase, []string{"go"})
	if err != nil {
		return err
	}
	doc, _, err := s.m3Scan(ctx, repo)
	if err != nil {
		return err
	}
	for _, check := range []string{"policy.hook-tests", "policy.codeowners-foreign"} {
		status := m3Status(doc, check)
		if status.Status != model.StatusNotApplicable.String() || !strings.Contains(status.Reason, "profile") {
			return fmt.Errorf("%s without a profile is %q (%s), want not applicable naming the profile: %w",
				check, status.Status, status.Reason, ErrAcceptance)
		}
	}
	return nil
}

// m3ViolationRow is the smallest proof of the required row: the check is
// measured clean on the rendered service, then the violation is applied
// and make -k ci runs every target whatever an earlier one answers, exits
// nonzero and names the check. The repository carries the row's name.
func (s *suite) m3ViolationRow(ctx context.Context, name string, violation m3Violation) error {
	repo, err := s.m3ServiceRepository(ctx, name, violation.capabilities)
	if err != nil {
		return err
	}
	doc, code, err := s.m3Scan(ctx, repo)
	if err != nil {
		return err
	}
	if err := m3JudgeClean(doc, violation.check); err != nil {
		return fmt.Errorf("the scan before the violation exited %d: %w", code, err)
	}
	if err := violation.apply(repo); err != nil {
		return err
	}
	return m3JudgeViolation(violation.check, s.m3Make(ctx, repo, "-k", "ci"))
}

// m3CIDefaultBranch records the checkout's HEAD as the remote default
// branch: the generated CI checks out with every branch fetched, and
// coverage-new reads its base there (decision C3 keeps a repository with no
// remote at exit 2), so the row's local repository takes the shape of a
// pipeline run on the default branch.
func m3CIDefaultBranch() []string {
	return []string{"update-ref", "refs/remotes/origin/main", m2Head}
}

// m3SurfaceSetup is the step the generated CI runs before make ci: the
// pinned surface linters of the languages the repository holds, because
// make ci composes lint-surface and a gate that cannot measure fails.
func m3SurfaceSetup() []string {
	return []string{m6SetupCommand, "--surface"}
}

// m3PassesCI runs the steps the generated CI ends with: golaw setup
// --surface, make ci, then git diff --exit-code, so a target that rewrites
// a tracked file fails the row as it fails the pipeline.
func (s *suite) m3PassesCI(ctx context.Context, repo string) error {
	if err := s.gitSteps(ctx, repo, m3CIDefaultBranch()); err != nil {
		return fmt.Errorf("record the default branch of the CI checkout: %w", err)
	}
	setup := s.execute(ctx, command{program: s.binary, args: m3SurfaceSetup(), dir: repo, env: s.env})
	if err := success(setup); err != nil {
		return fmt.Errorf("install the surface tools as the generated CI does: %w", err)
	}
	got := s.m3Make(ctx, repo, "ci")
	if got.err != nil {
		return fmt.Errorf("execute make ci: %w", got.err)
	}
	if got.code != 0 {
		return fmt.Errorf("make ci exit %d: %s: %w", got.code, m3Tail(got), ErrAcceptance)
	}
	if err := s.gitSteps(ctx, repo, []string{"diff", "--exit-code"}); err != nil {
		return fmt.Errorf("make ci changed a tracked file: %w", err)
	}
	return nil
}

// m3Tail is the end of a process output, the part that names what failed.
func m3Tail(got result) string {
	text := strings.TrimSpace(strings.ReplaceAll(got.stdout+"\n"+got.stderr, "\r\n", "\n"))
	lines := strings.Split(text, "\n")
	if len(lines) > m3TailLines {
		lines = lines[len(lines)-m3TailLines:]
	}
	return strings.Join(lines, "\n")
}
