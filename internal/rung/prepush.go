package rung

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/vuln"
)

// scanner names the one tool of the pre-push rung, and shortTip is how much
// of a pushed object name a line shows.
const (
	scanner  = "govulncheck"
	shortTip = 12
)

// RunPrePush is the pre-push rung (section 7, A6): govulncheck alone over
// the tree of every pushed tip that is not a deletion, its reachable
// advisories judged by the dated waivers of the policy in force on the day
// of deps.Now. It refuses a reachable advisory no waiver holds and a
// scanner of another version; anything else that keeps the scan from a
// verdict, a scanner that cannot start included, is the one nonblocking
// Unmeasured of A6, named in a line
// while the push goes on, because the pipeline judges it. The URL git
// passes beside the remote is not read: the rung judges the pushed trees,
// and a URL may carry a credential no line should print.
func RunPrePush(ctx context.Context, deps Dependencies, remote, _ string) Outcome {
	a, out, ok := gate(ctx, deps, PrePush)
	if !ok {
		return out
	}
	updates, err := runner.ParseRefUpdates(deps.Stdin)
	if err != nil {
		return unmeasuredPush("the ref lines git gave", err)
	}
	ranges, err := runner.Ranges(ctx, deps.Dir, remote, updates, deps.Environ)
	if err != nil {
		return unmeasuredPush("the pushed refs", err)
	}
	push := prepush{deps: deps, waivers: a.loaded.Policy.Waivers}
	result := Outcome{}
	for _, tip := range push.commits(ctx, tips(ranges)) {
		result = join(result, push.scan(ctx, tip))
	}
	if result.Code != codeClean {
		result.Lines = append(result.Lines, refused(a.loaded.Policy.Language, PrePush))
	}
	return result
}

// prepush is one run of the pre-push rung.
type prepush struct {
	deps    Dependencies
	waivers []model.Waiver
}

// tips are the pushed tips that are not deletions, each once, in order.
func tips(ranges []runner.PushRange) []string {
	var out []string
	for _, r := range ranges {
		if !r.Delete && !slices.Contains(out, r.Tip) {
			out = append(out, r.Tip)
		}
	}
	return out
}

// commits resolves each pushed tip to the commit it names, each commit
// once, in order: the object of an annotated tag is never HEAD, so a
// release push of a branch and its tag would scan one tree twice, once in a
// copy. A tip git cannot resolve stays as it is and its scan names why.
func (p prepush) commits(ctx context.Context, tips []string) []string {
	var out []string
	for _, tip := range tips {
		commit := tip
		if res, err := plumbing(ctx, p.deps, "rev-parse", "--verify", "--quiet", "--end-of-options", tip+"^{commit}"); err == nil && res.Code == 0 {
			commit = strings.TrimSpace(string(res.Stdout))
		}
		if !slices.Contains(out, commit) {
			out = append(out, commit)
		}
	}
	return out
}

// scan runs govulncheck over the tree of one tip and judges what it
// reports.
func (p prepush) scan(ctx context.Context, tip string) Outcome {
	short := tip[:min(len(tip), shortTip)]
	dir, closeTree, noted, err := p.tree(ctx, tip)
	if err != nil {
		return join(noted, unmeasuredPush(short, err))
	}
	out, err := runner.Run(ctx, runner.Command{Name: scanner, Args: []string{"-format", "json", "./..."}, Dir: dir, Env: runner.UserEnv(p.deps.Environ)})
	result := join(noted, p.read(short, out, err))
	if closed := closeTree(); closed != nil {
		result = join(result, unmeasuredPush(short, closed))
	}
	return result
}

// read judges what govulncheck answered for one tip.
func (p prepush) read(short string, out runner.Output, err error) Outcome {
	tool, _ := runner.LookupTool(scanner)
	switch {
	case errors.Is(err, runner.ErrStart):
		return unmeasuredPush(short, fmt.Errorf("%w: install %s %s with golaw setup", err, scanner, tool.Version))
	case err != nil:
		return unmeasuredPush(short, err)
	case out.Code != 0:
		return unmeasuredPush(short, fmt.Errorf("%w: exit status %d: %s", ErrScan, out.Code, firstLine(out.Stderr)))
	}
	version, err := vuln.Scanner(bytes.NewReader(out.Stdout))
	found, readErr := vuln.Reachable(bytes.NewReader(out.Stdout))
	if err = errors.Join(err, readErr); err != nil {
		return unmeasuredPush(short, err)
	}
	if !tool.Matches(version) {
		return Outcome{Code: codeFailure, Lines: []string{fmt.Sprintf("%s%s: %v: %s answers %s and the pin is %s: run golaw setup", prefix(PrePush), short, runner.ErrToolVersion, scanner, version, tool.Version)}}
	}
	return p.judge(short, found)
}

// judge splits the reachable advisories of one tip by the waivers and
// names the entry behind each verdict: for a waived one the entry in force,
// because an extension may sit beside the entry that lapsed and a past day
// would tell the person the waiver no longer holds; for a refused one its
// lapsed entry, the one to renew.
func (p prepush) judge(short string, found []vuln.Finding) Outcome {
	if len(found) == 0 {
		return Outcome{Lines: []string{prefix(PrePush) + short + ": govulncheck finds no reachable vulnerability"}}
	}
	verdict := vuln.Judge(found, p.waivers, p.deps.Now)
	lapsed := vuln.Expired(p.waivers, p.deps.Now)
	out := Outcome{}
	for _, f := range verdict.Waived {
		w := p.waivers[slices.IndexFunc(p.waivers, func(w model.Waiver) bool { return w.ID == f.ID && !slices.Contains(lapsed, w) })]
		out.Lines = append(out.Lines, fmt.Sprintf("%s%s: %s waived until %s (%s)", prefix(PrePush), short, f.ID, w.Until, w.Record))
	}
	for _, f := range verdict.Refused {
		line := fmt.Sprintf("%s%s: %s reachable in %s, fixed in %s", prefix(PrePush), short, f.ID, f.Module, fixedIn(f.Fixed))
		if i := slices.IndexFunc(p.waivers, func(w model.Waiver) bool { return w.ID == f.ID }); i >= 0 {
			line += fmt.Sprintf("; its waiver expired on %s (%s)", p.waivers[i].Until, p.waivers[i].Record)
		}
		out.Code, out.Lines = codeFindings, append(out.Lines, line)
	}
	return out
}

// tree answers where govulncheck reads a tip: the worktree in place when
// it holds the tip, HEAD with nothing changed or untracked and no Go file
// git ignores (go would read it, the push carries none), a copy of the tip
// under the state directory otherwise, with its removal; a sweep of the
// trees an ended rung left that fails is named and the scan goes on.
func (p prepush) tree(ctx context.Context, tip string) (string, func() error, Outcome, error) {
	head, err := plumbing(ctx, p.deps, "rev-parse", "--verify", "-q", "HEAD^{commit}")
	if err != nil {
		return "", nil, Outcome{}, err
	}
	status, err := plumbing(ctx, p.deps, "--no-optional-locks", "status", "--porcelain", "-z", "--untracked-files=all")
	if err != nil {
		return "", nil, Outcome{}, err
	}
	ignored, err := ignoredGo(ctx, p.deps)
	if err != nil {
		return "", nil, Outcome{}, err
	}
	if strings.TrimSpace(string(head.Stdout)) == tip && status.Code == 0 && len(status.Stdout) == 0 && len(ignored) == 0 {
		return p.deps.Dir, func() error { return nil }, Outcome{}, nil
	}
	noted := Outcome{}
	if swept := runner.Sweep(treesDir(p.deps)); swept != nil {
		noted = Outcome{Lines: []string{prefix(PrePush) + "could not sweep the trees an ended rung left: " + swept.Error()}}
	}
	copied, err := runner.Materialize(ctx, runner.MaterializeInput{Dir: p.deps.Dir, Env: p.deps.Environ, Rev: tip, Parent: treesDir(p.deps), Bounds: runner.DefaultBounds()})
	if err != nil {
		return "", nil, noted, fmt.Errorf("copy the tree of the tip: %w", err)
	}
	return copied.Dir, copied.Close, noted, nil
}

// unmeasuredPush is the nonblocking Unmeasured of A6: a line that names
// what kept the scan from a verdict, and the push going on.
func unmeasuredPush(what string, err error) Outcome {
	return Outcome{Lines: []string{fmt.Sprintf("%s%s: govulncheck could not judge it, the pipeline judges it: %v", prefix(PrePush), what, err)}}
}

// fixedIn still names an advisory without a fix, the case a dated waiver
// exists for.
func fixedIn(version string) string {
	if version == "" {
		return "no version yet"
	}
	return version
}

// firstLine keeps one line of what a failed scan printed, because the
// push goes on and the pipeline repeats the scan with all of it.
func firstLine(data []byte) string {
	line, _, _ := strings.Cut(strings.TrimSpace(string(data)), "\n")
	return line
}
