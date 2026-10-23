package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/cristianargotti/golaw/internal/coverage"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/source"
	"github.com/cristianargotti/golaw/internal/vuln"
)

// seededOnce is the refusal of a second seed (A24): the debt set is seeded
// by the first adoption alone and only shrinks afterwards.
const seededOnce = "the debt set is seeded once (A24)"

// seedOnce refuses a seed once the repository holds a policy: in the
// checkout, at HEAD, or in an enrollment record of this machine. The
// uncommitted policy a plain init --apply wrote and enrolled is the one
// state a person can still start over from, so its refusal names how:
// the re-enrollment of a seeded policy over it would broaden what the
// record enrolled, which CheckReenrollment refuses (A24).
func (b binding) seedOnce(ctx context.Context, root string, current []byte) error {
	_, atHead, err := policy.ReadHead(ctx, root)
	if err != nil {
		return initOperation("read the policy at HEAD", err)
	}
	dirs, err := b.dirs()
	if err != nil {
		return err
	}
	record, enrolled, err := policy.ReadRecord(dirs.State, root)
	if err != nil {
		return initOperation("read the enrollment record", err)
	}
	switch {
	case current == nil && !atHead && !enrolled:
		return nil
	case current != nil && !atHead && enrolled && record.WrittenBy == policy.WrittenByInit:
		return fmt.Errorf("%w: %s; %s of %s is the uncommitted policy of a plain init --apply: delete it and %s, then run golaw init --seed-debt --apply",
			model.ErrUsage, seededOnce, policy.FileName, root, policy.RecordPath(dirs.State, root))
	}
	return fmt.Errorf("%w: %s; remove entries from golaw.yml, never add them", model.ErrUsage, seededOnce)
}

// seedRecordAbsent refuses a seed whose record is already in the tree.
func (b binding) seedRecordAbsent(root, record string) error {
	_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(record)))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return initOperation("inspect the debt record", err)
	default:
		return fmt.Errorf("%w: %s: %s exists", model.ErrUsage, seededOnce, record)
	}
}

// coverageExemptions are the files the cover profile measures under the
// file floor of the policy init writes: the exemptions that floor allows.
// A file the profile does not measure is no exemption, because the
// coverage law fails it whatever the list says.
func coverageExemptions(profile, root string, compiled *model.EffectivePolicy) ([]string, error) {
	if profile == "" {
		return nil, nil
	}
	report, err := evaluateCoverageFile(profile, root, coverage.FloorsFrom(compiled.OptionsFor(policy.CoverageCheck)))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, failure := range report.Failures {
		measured := slices.ContainsFunc(report.Files, func(m coverage.Measure) bool { return m.Name == failure.Name })
		if measured && failure.Statements > 0 {
			out = append(out, failure.Name)
		}
	}
	return out, nil
}

// reachableAdvisories are the advisory ids a govulncheck JSON stream
// reports reachable from the code.
func reachableAdvisories(stream string) ([]string, error) {
	if stream == "" {
		return nil, nil
	}
	file, err := os.Open(stream) //nolint:gosec // the person names the stream to read
	if err != nil {
		return nil, fmt.Errorf("open the vulnerability stream %q: %w", stream, err)
	}
	found, err := vuln.Reachable(file)
	if err = errors.Join(err, file.Close()); err != nil {
		return nil, fmt.Errorf("read the vulnerability stream %q: %w", stream, err)
	}
	ids := make([]string, 0, len(found))
	for _, f := range found {
		ids = append(ids, f.ID)
	}
	return ids, nil
}

// seedFindings dispatches the catalog in scan mode over the worktree with
// the files given folded in: over every file, or over the targets named,
// whose findings alone it answers.
func seedFindings(ctx context.Context, s seedRun, files map[string][]byte, targets []string) ([]model.Finding, error) {
	muts := make([]source.Mutation, 0, len(files))
	for name, content := range files {
		muts = append(muts, source.Mutation{Path: name, Op: model.CandidateReplace, Content: content})
	}
	slices.SortFunc(muts, func(x, y source.Mutation) int { return cmp.Compare(x.Path, y.Path) })
	overlay, _, err := source.NewOverlay(s.snapshot, muts)
	if err != nil {
		return nil, initOperation("fold the files init writes", err)
	}
	judged := func(name string) bool { return targets == nil || slices.Contains(targets, name) }
	run, err := engine.Dispatch(ctx, s.registry, model.Input{Mode: model.ModeScan, Policy: s.policy, Snapshot: overlay, Targets: overlay.Select(judged)})
	if err != nil {
		return nil, initOperation("scan the debt to seed", err)
	}
	return slices.DeleteFunc(run.Findings(), func(f model.Finding) bool { return !judged(f.Path) }), nil
}

// debtCounts are the sizes of the exception, exemption and waiver lists
// of a seeded policy, which ApplyDebt already decoded once.
func debtCounts(data []byte) [3]int {
	f, err := policy.Decode(data)
	if err != nil || f.Checks.Coverage == nil {
		return [3]int{}
	}
	return [3]int{len(f.Exceptions), len(f.Checks.Coverage.FileExemptions), len(f.Waivers)}
}
