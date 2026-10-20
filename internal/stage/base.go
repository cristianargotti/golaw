package stage

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"slices"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/report"
	"github.com/cristianargotti/golaw/internal/source"
)

// countRun is a count a goast message carries, the digits before the unit
// they count ("file has 321 lines", "function long has 44 statements", "a
// string literal appears 4 times", "13 non-test files in one adapter
// package"); any other digit, as one of a name, tells two findings apart.
var countRun = regexp.MustCompile(`\b\d+( lines| statements| times| non-test files)\b`)

// introducedOnly keeps, in each outcome of the run over the index, the
// findings the same files at the base do not hold, compared as a multiset
// of staged keys: the base is the tree of the base commit, the empty tree
// on an unborn branch, and a renamed file is read at its old path there, so
// a finding the commit only carries along is never its own.
func introducedOnly(ctx context.Context, in Input, candidate model.Input, run *engine.Run) error {
	moved := renames(candidate.Changes)
	held, err := judgeBase(ctx, in, candidate, baseOf(in), moved)
	if err != nil {
		return err
	}
	for i := range run.Outcomes {
		got, was := &run.Outcomes[i].Result, held.staged.Outcomes[i].Result
		got.Findings = provable(got, introduced(got.Findings, held.findings(i)), was, moved)
	}
	return nil
}

// baseRun is the base judged in two runs: over the paths the targets had
// there, and over the files beside a target whose directory the first run
// holds no file of.
type baseRun struct {
	staged *engine.Run
	beside *engine.Run // nil when the first run holds a file of every directory a target sits in
	moved  map[string]string
}

// findings answers what the base holds for outcome i, as the index names
// its paths.
func (b *baseRun) findings(i int) []model.Finding {
	found := b.staged.Outcomes[i].Result.Findings
	if b.beside != nil {
		found = slices.Concat(found, b.beside.Outcomes[i].Result.Findings)
	}
	return rekeyed(found, b.moved)
}

// judgeBase freezes the base at rev and judges the paths the targets had
// there, then, in a run of their own, the other files of every directory a
// target joins that the first run holds no file of: a rule that judges a
// directory through one of its files, as the file count of an adapter
// package, is asked about a directory the commit only adds to. Only the
// first run can hold a finding back (provable), so a file beside that no
// check can judge never makes a staged finding unmeasured.
func judgeBase(ctx context.Context, in Input, candidate model.Input, rev string, moved map[string]string) (*baseRun, error) {
	base, err := source.NewRevision(ctx, in.Repo, rev)
	if err != nil {
		return nil, fmt.Errorf("freeze the base of %s: %w", in.Repo.Root, err)
	}
	paths := basePaths(candidate.Targets, moved)
	targets := base.Select(func(p string) bool { return paths[p] })
	held := &baseRun{moved: moved}
	if held.staged, err = judge(ctx, in, base, targets); err != nil {
		return nil, err
	}
	dirs := joined(candidate.Targets, targets)
	if len(dirs) == 0 {
		return held, nil
	}
	if beside := base.Select(func(p string) bool { return dirs[path.Dir(p)] }); len(beside) > 0 {
		if held.beside, err = judge(ctx, in, base, beside); err != nil {
			return nil, err
		}
	}
	return held, nil
}

// judge dispatches the registry over files of the base in ModeStaged,
// under the policy in force, as the index is judged.
func judge(ctx context.Context, in Input, base model.Snapshot, targets []model.File) (*engine.Run, error) {
	run, err := engine.Dispatch(ctx, in.Registry, model.Input{Mode: model.ModeStaged, Policy: in.Loaded.Policy, Snapshot: base, Targets: targets})
	if err != nil {
		return nil, fmt.Errorf("judge the base of %s: %w", in.Repo.Root, err)
	}
	return run, nil
}

// joined names the directories the targets of the index sit in where the
// targets of the base hold no file, the ones a directory rule would
// otherwise never be asked about at the base.
func joined(targets, base []model.File) map[string]bool {
	dirs := map[string]bool{}
	for _, f := range targets {
		dirs[path.Dir(f.Path)] = true
	}
	for _, f := range base {
		delete(dirs, path.Dir(f.Path))
	}
	return dirs
}

// introduced is report.Introduced under stagedKey: the index findings the
// base does not hold, each kept finding answered with the fingerprint the
// engine gave it, which an exception that names its record in the message
// afterwards does not change. report.Introduced keeps the candidate order,
// so the kept findings are found again in order among the keyed ones.
func introduced(candidate, base []model.Finding) []model.Finding {
	keys := keyed(candidate)
	kept := report.Introduced(keys, keyed(base))
	for i, j := 0, 0; i < len(keys) && j < len(kept); i++ {
		if keys[i] == kept[j] {
			kept[j].Fingerprint = candidate[i].Fingerprint
			j++
		}
	}
	return kept
}

// keyed copies findings with their staged key in place of the fingerprint.
func keyed(findings []model.Finding) []model.Finding {
	out := slices.Clone(findings)
	for i := range out {
		out[i].Fingerprint = stagedKey(out[i])
	}
	return out
}

// stagedKey is model.Fingerprint with every count of the message read as
// one mark: an edit changes the count while the finding stays the one the
// base held, so a count never makes it new.
func stagedKey(f model.Finding) string {
	f.Message = countRun.ReplaceAllString(f.Message, "#$1")
	return model.Fingerprint(f)
}

// provable holds back the introduced findings the base cannot disprove:
// when the base left a path over the read bound, or stopped before it
// measured, a finding there may be one the base held, so it is neither the
// commit's own nor clean. The outcome is then unmeasured with the base's
// reason and those paths among its unread files, which a rung names and
// leaves to the pipeline (section 5.2).
func provable(got *model.Result, fresh []model.Finding, base model.Result, moved map[string]string) []model.Finding {
	if base.Status.Kind != model.StatusUnmeasured || got.Status.Kind != model.StatusMeasured {
		return fresh
	}
	unread := rekeyedPaths(base.Unread, moved)
	doubted := func(f model.Finding) bool { return len(unread) == 0 || slices.Contains(unread, f.Path) }
	kept := slices.DeleteFunc(slices.Clone(fresh), doubted)
	if len(kept) == len(fresh) {
		return fresh
	}
	got.Status = model.Unmeasured("compare with the base: " + base.Status.Reason)
	for _, f := range fresh {
		if slices.Contains(unread, f.Path) && !slices.Contains(got.Unread, f.Path) {
			got.Unread = append(got.Unread, f.Path)
		}
	}
	return kept
}

// baseOf is the commit the changes were taken against: the base the input
// names, HEAD otherwise, empty on an unborn branch.
func baseOf(in Input) string {
	if in.Index.Base != "" {
		return in.Index.Base
	}
	return in.Repo.Head
}

// renames maps the old path of every renamed file to its new path.
func renames(changes []model.Change) map[string]string {
	moved := map[string]string{}
	for _, c := range changes {
		if c.Op == model.ChangeRename {
			moved[c.OldPath] = c.NewPath
		}
	}
	return moved
}

// basePaths names, for each target, the path its bytes had at the base:
// the old path of a renamed file, the same path otherwise.
func basePaths(targets []model.File, moved map[string]string) map[string]bool {
	from := make(map[string]string, len(moved))
	for old, renamed := range moved {
		from[renamed] = old
	}
	paths := make(map[string]bool, len(targets))
	for _, f := range targets {
		if old, ok := from[f.Path]; ok {
			paths[old] = true
			continue
		}
		paths[f.Path] = true
	}
	return paths
}

// rekeyed answers the findings of the base as the index names them: a
// finding at the old path of a renamed file moves to its new path, with
// the fingerprint of that path, so a file that only moved introduces
// nothing.
func rekeyed(findings []model.Finding, moved map[string]string) []model.Finding {
	out := make([]model.Finding, 0, len(findings))
	for _, f := range findings {
		if renamed, ok := moved[f.Path]; ok {
			f.Path = renamed
			f.Fingerprint = model.Fingerprint(f)
		}
		out = append(out, f)
	}
	return out
}

// rekeyedPaths names paths of the base as the index names them: the new
// path of a renamed file.
func rekeyedPaths(paths []string, moved map[string]string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if renamed, ok := moved[p]; ok {
			p = renamed
		}
		out = append(out, p)
	}
	return out
}
