package surface_test

import (
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// tree is the frozen in-memory snapshot of the files.
func tree(files map[string]string) *proof.Tree {
	content := make(map[string][]byte, len(files))
	for path, text := range files {
		content[path] = []byte(text)
	}
	return proof.NewTree(content)
}

// scanInput is the scan of every file under the proof policy.
func scanInput(c model.Check, files map[string]string) model.Input {
	snapshot := tree(files)
	return model.Input{
		Mode: model.ModeScan, Policy: proof.Policy(), Snapshot: snapshot,
		Targets: snapshot.Files(), Options: c.Spec().Options.Defaults(),
	}
}

// based folds the candidate tree over its base, as the overlay of a hook
// does.
type based struct {
	*proof.Tree

	base *proof.Tree
}

func (b based) Base() model.Snapshot { return b.base }

// writeInput is the pre write event that turns base into base plus the
// candidate files; every candidate is reconstructed unless partial names it.
func writeInput(c model.Check, base, candidate map[string]string, partial ...string) model.Input {
	merged := map[string]string{}
	maps.Copy(merged, base)
	maps.Copy(merged, candidate)
	snapshot := based{Tree: tree(merged), base: tree(base)}
	event := &model.Event{Shape: model.ShapeWrite, Agent: "tester", Tool: "Write"}
	for _, path := range slices.Sorted(maps.Keys(candidate)) {
		op := model.CandidateCreate
		if _, held := base[path]; held {
			op = model.CandidateReplace
		}
		event.Candidates = append(event.Candidates, model.Candidate{Path: path, Op: op, Reconstructed: !slices.Contains(partial, path)})
	}
	return model.Input{
		Mode: model.ModeHook, Policy: proof.Policy(), Snapshot: snapshot, Event: event,
		Targets: model.Select(snapshot, func(p string) bool { _, held := candidate[p]; return held }),
		Options: c.Spec().Options.Defaults(),
	}
}

// postInput is the post event after the files were written: the snapshot
// holds them and no base.
func postInput(c model.Check, files map[string]string) model.Input {
	snapshot := tree(files)
	post := &model.PostResult{Changed: slices.Sorted(maps.Keys(files))}
	return model.Input{
		Mode: model.ModeHook, Policy: proof.Policy(), Snapshot: snapshot, Targets: snapshot.Files(),
		Event:   &model.Event{Shape: model.ShapePost, Agent: "tester", Tool: "Write", Post: post},
		Options: c.Spec().Options.Defaults(),
	}
}

// run runs the check and fails the test unless it measured.
func run(t *testing.T, c model.Check, in model.Input) model.Result {
	t.Helper()
	res, err := c.Run(t.Context(), in)
	if err != nil {
		t.Fatalf("%s: %v", c.Spec().ID, err)
	}
	if res.Status.Kind != model.StatusMeasured {
		t.Fatalf("%s: status %s (%s), want measured", c.Spec().ID, res.Status.Kind, res.Status.Reason)
	}
	return res
}

// scanFiles runs the check over the files in a scan.
func scanFiles(t *testing.T, c model.Check, files map[string]string) model.Result {
	t.Helper()
	return run(t, c, scanInput(c, files))
}

// found renders each finding as path:line code, in the order the check
// gave them.
func found(res model.Result) []string {
	out := make([]string, 0, len(res.Findings))
	for i := range res.Findings {
		f := &res.Findings[i]
		out = append(out, fmt.Sprintf("%s:%d %s", f.Path, f.Line, f.Code))
	}
	return out
}

// expectFound fails unless the findings are exactly these, in order.
func expectFound(t *testing.T, res model.Result, want ...string) {
	t.Helper()
	if got := found(res); !slices.Equal(got, want) {
		t.Fatalf("findings = %q, want %q", got, want)
	}
}

// sameFinding compares what a check fills: the engine adds the check id
// and the rule, and the column is the lexer's business.
func sameFinding(a, b model.Finding) bool {
	return a.Path == b.Path && a.Line == b.Line && a.Code == b.Code && a.Message == b.Message &&
		a.Sonar == b.Sonar && a.Severity == b.Severity
}

// fileCase is one file and the findings a check gives it in a scan.
type fileCase struct {
	name string
	path string
	src  string
	want []string
}

// runCases scans each case alone.
func runCases(t *testing.T, c model.Check, cases []fileCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			expectFound(t, scanFiles(t, c, map[string]string{tc.path: tc.src}), tc.want...)
		})
	}
}
