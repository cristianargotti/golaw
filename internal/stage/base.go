package stage

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

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

// packageRule reports one finding for a package at the first of its files
// a run meets, so the base and the index meet it at different files.
const packageRule = "goast.goleak"

// goSuffix ends the name of a Go file.
const goSuffix = ".go"

// introducedOnly keeps, in each outcome of the run over the index, the
// findings no base holds, compared as a multiset of staged keys: the tree
// of the base commit, the empty tree on an unborn branch, and while a merge
// is in progress the tree of theirs, the other parent, since a finding the
// other side holds is one the merge brought (section 8.1). A renamed file
// is read at its old path in each, so a finding the commit only carries
// along is never its own. It answers the status of each outcome at the
// base, which the pre-commit rung compares its stated limits with.
func introducedOnly(ctx context.Context, in Input, candidate model.Input, run *engine.Run, theirs string) ([]model.Status, error) {
	moved := renames(candidate.Changes)
	revs := []string{baseOf(in)}
	if theirs != "" {
		revs = append(revs, theirs)
	}
	bases := make([]*baseRun, 0, len(revs))
	for _, rev := range revs {
		held, err := judgeBase(ctx, in, candidate, rev, moved)
		if err != nil {
			return nil, err
		}
		bases = append(bases, held)
	}
	statuses := make([]model.Status, 0, len(run.Outcomes))
	for i := range bases[0].staged.Outcomes {
		statuses = append(statuses, bases[0].staged.Outcomes[i].Result.Status)
	}
	for i := range run.Outcomes {
		got := &run.Outcomes[i].Result
		got.Findings = introduced(got.Findings, heldAt(bases, i))
		for _, held := range bases {
			held.provable(got, held.staged.Outcomes[i].Result)
			held.besideDoubt(got, i)
		}
	}
	return statuses, nil
}

// heldAt is what the bases hold for outcome i, keyed, as a multiset union:
// a finding both parents of a merge hold cancels one finding of the index,
// never two.
func heldAt(bases []*baseRun, i int) []model.Finding {
	held := keyed(bases[0].findings(i))
	for _, b := range bases[1:] {
		held = append(held, report.Introduced(keyed(b.findings(i)), held)...)
	}
	return held
}

// baseRun keeps the two runs of a base apart: the files beside a target
// only add the findings a directory rule makes there, while the run over
// the paths the targets had decides what the base cannot disprove.
type baseRun struct {
	staged *engine.Run
	beside *engine.Run       // nil when no file of the base sits beside a target
	holds  map[string]string // the base path of each file the first run judges, by its path in the index
	moved  map[string]string
}

// findings joins what both runs of the base hold for outcome i, under the
// paths of the index, so a directory finding of the files beside a target
// counts like any other.
func (b *baseRun) findings(i int) []model.Finding {
	found := b.staged.Outcomes[i].Result.Findings
	if b.beside != nil {
		found = slices.Concat(found, b.beside.Outcomes[i].Result.Findings)
	}
	return rekeyed(found, b.moved)
}

// judgeBase freezes the base at rev and judges the paths the targets had
// there, then, in a run of their own, the other files of every directory a
// target joins that the first run holds no file of, and the other Go files
// of the package of every Go target: a rule that judges a directory
// through one of its files, as the file count of an adapter package or a
// package that starts goroutines, is asked about the whole directory the
// commit adds to. Only the first run can hold a finding back (provable),
// so a file beside that no check can judge never makes a staged finding
// unmeasured.
func judgeBase(ctx context.Context, in Input, candidate model.Input, rev string, moved map[string]string) (*baseRun, error) {
	base, err := source.NewRevision(ctx, in.Repo, rev)
	if err != nil {
		return nil, fmt.Errorf("freeze the base of %s: %w", in.Repo.Root, err)
	}
	paths := basePaths(candidate.Targets, moved)
	targets := base.Select(func(p string) bool { return paths[p] })
	held := &baseRun{holds: make(map[string]string, len(targets)), moved: moved}
	for _, f := range targets {
		held.holds[renamedTo(f.Path, moved)] = f.Path
	}
	if held.staged, err = judge(ctx, in, base, targets); err != nil {
		return nil, err
	}
	dirs, packages := joined(candidate.Targets, targets), goPackages(candidate.Targets)
	besides := func(p string) bool {
		return dirs[path.Dir(p)] || !paths[p] && packages[path.Dir(p)] && strings.HasSuffix(p, goSuffix)
	}
	if beside := base.Select(besides); len(beside) > 0 {
		if held.beside, err = judge(ctx, in, base, beside); err != nil {
			return nil, err
		}
	}
	return held, nil
}

// judge runs the registry over files of a base in the mode and under the
// policy the index is judged in, so the findings of both sides compare.
func judge(ctx context.Context, in Input, base model.Snapshot, targets []model.File) (*engine.Run, error) {
	run, err := engine.Dispatch(ctx, in.Registry, model.Input{Mode: model.ModeStaged, Policy: in.Loaded.Policy, Snapshot: base, Targets: targets})
	if err != nil {
		return nil, fmt.Errorf("judge the base of %s: %w", in.Repo.Root, err)
	}
	return run, nil
}

// goPackages names the directories of the Go targets of the index, the
// packages a package rule judges whole.
func goPackages(targets []model.File) map[string]bool {
	dirs := map[string]bool{}
	for _, f := range targets {
		if strings.HasSuffix(f.Path, goSuffix) {
			dirs[path.Dir(f.Path)] = true
		}
	}
	return dirs
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
// keyed findings of the bases do not hold, each kept finding answered with
// the fingerprint the engine gave it, which an exception that names its
// record in the message afterwards does not change. report.Introduced
// keeps the candidate order, so the kept findings are found again in order
// among the keyed ones.
func introduced(candidate, held []model.Finding) []model.Finding {
	keys := keyed(candidate)
	kept := report.Introduced(keys, held)
	for i, j := 0, 0; i < len(keys) && j < len(kept); i++ {
		if keys[i] == kept[j] {
			kept[j].Fingerprint = candidate[i].Fingerprint
			j++
		}
	}
	return kept
}

// keyed hands report.Introduced, which compares fingerprints, copies that
// carry the staged key instead, so a finding whose count alone changed
// matches the one the base held while the run keeps the fingerprints the
// engine gave.
func keyed(findings []model.Finding) []model.Finding {
	out := slices.Clone(findings)
	for i := range out {
		out[i].Fingerprint = stagedKey(out[i])
	}
	return out
}

// stagedKey is model.Fingerprint with every count of the message read as
// one mark: an edit changes the count while the finding stays the one the
// base held, so a count never makes it new. A finding of a package rule is
// keyed by its package: it sits at the first file a run meets, which is
// the staged file in the index and any file of the package at the base.
func stagedKey(f model.Finding) string {
	f.Message = countRun.ReplaceAllString(f.Message, "#$1")
	if f.Check == packageRule {
		f.Path = path.Dir(f.Path)
	}
	return model.Fingerprint(f)
}

// baseOf is the commit the changes were taken against: the base the input
// names, HEAD otherwise, empty on an unborn branch.
func baseOf(in Input) string {
	if in.Index.Base != "" {
		return in.Index.Base
	}
	return in.Repo.Head
}

// renames maps the old path of every renamed file to its new path, so a
// moved file is read at its old path at the base and its findings there
// meet the ones the index reports at the new path.
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

// renamedTo names a path of the base by its path in the index, so what a
// rung names after a rename is a file the developer has.
func renamedTo(p string, moved map[string]string) string {
	if renamed, ok := moved[p]; ok {
		return renamed
	}
	return p
}
