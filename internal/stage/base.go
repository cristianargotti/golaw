package stage

import (
	"context"
	"fmt"
	"regexp"
	"slices"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/report"
	"github.com/cristianargotti/golaw/internal/source"
)

// digitRun is a run of digits, a count a message carries.
var digitRun = regexp.MustCompile(`\d+`)

// introducedOnly keeps, in each outcome of the run over the index, the
// findings the same files at the base do not hold, compared as a multiset
// of staged keys: the base is the tree of the base commit, the empty tree
// on an unborn branch, and a renamed file is read at its old path there, so
// a finding the commit only carries along is never its own.
func introducedOnly(ctx context.Context, in Input, candidate model.Input, run *engine.Run) error {
	base, err := source.NewRevision(ctx, in.Repo, baseOf(in))
	if err != nil {
		return fmt.Errorf("freeze the base of %s: %w", in.Repo.Root, err)
	}
	moved := renames(candidate.Changes)
	paths := basePaths(candidate.Targets, moved)
	before := model.Input{
		Mode: model.ModeStaged, Policy: candidate.Policy, Snapshot: base,
		Targets: base.Select(func(path string) bool { return paths[path] }),
	}
	held, err := engine.Dispatch(ctx, in.Registry, before)
	if err != nil {
		return fmt.Errorf("judge the base of %s: %w", in.Repo.Root, err)
	}
	for i := range run.Outcomes {
		got, was := &run.Outcomes[i].Result, held.Outcomes[i].Result
		got.Findings = provable(got, introduced(got.Findings, rekeyed(was.Findings, moved)), was, moved)
	}
	return nil
}

// introduced is report.Introduced under stagedKey: the index findings the
// base does not hold, each kept finding answered with its own fingerprint,
// which every finding carries as model.Fingerprint makes it.
func introduced(candidate, base []model.Finding) []model.Finding {
	kept := report.Introduced(keyed(candidate), keyed(base))
	for i := range kept {
		kept[i].Fingerprint = model.Fingerprint(kept[i])
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

// stagedKey is model.Fingerprint with every run of digits in the message
// read as one mark: a goast message carries a count ("file has 321 lines",
// "a string literal appears 4 times") that an edit changes while the
// finding stays the one the base held, so a count never makes it new.
func stagedKey(f model.Finding) string {
	f.Message = digitRun.ReplaceAllString(f.Message, "#")
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
