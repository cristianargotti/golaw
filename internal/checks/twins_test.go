package checks_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/checks/text"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
	"github.com/cristianargotti/golaw/internal/surfacefix"
)

// committedTwin is a snapshot of a twin committed as a repository holds it.
func committedTwin(t *testing.T, twin surfacefix.Twin) model.Snapshot {
	t.Helper()
	repo := gittest.New(t)
	if err := surfacefix.Write(repo.Root, twin); err != nil {
		t.Fatal(err)
	}
	repo.Git("add", "--all")
	repo.Git("commit", "--quiet", "--message", "test: the fixture twin")
	found, err := source.Discover(t.Context(), repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := source.NewWorktree(t.Context(), found)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

// TestTheFixtureTwinsMeetTheTextLaws keeps the fixture repository of M6
// honest about what it reproduces: both twins carry no dash, no trace term
// and no tool name, so every finding a later check reports on them is the
// issue class it claims.
func TestTheFixtureTwinsMeetTheTextLaws(t *testing.T) {
	t.Parallel()
	reg, err := checks.Build(text.Checks())
	if err != nil {
		t.Fatal(err)
	}
	for name, twin := range map[string]surfacefix.Twin{"red": surfacefix.Red, "corrected": surfacefix.Corrected} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			snapshot := committedTwin(t, twin)
			policy := model.CoreDefaults()
			in := model.Input{Mode: model.ModeScan, Policy: &policy, Snapshot: snapshot, Targets: snapshot.Files()}
			run, err := engine.Dispatch(t.Context(), reg, in)
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range run.Findings() {
				if strings.HasPrefix(f.Check, "text.") {
					t.Errorf("%s:%d %s %s", f.Path, f.Line, f.Check, f.Message)
				}
			}
		})
	}
}

// TestTheRedTwinHoldsEveryExpectedFileAsText proves each expectation names
// a file a check reads.
func TestTheRedTwinHoldsEveryExpectedFileAsText(t *testing.T) {
	t.Parallel()
	snapshot := committedTwin(t, surfacefix.Red)
	for _, e := range surfacefix.Expectations() {
		if class := snapshot.Class(e.Path); !class.IsText() {
			t.Errorf("%s is %s, want a text file the checks read", e.Path, class)
		}
	}
}
