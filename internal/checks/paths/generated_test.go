package paths_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/paths"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// broken is a snapshot whose every read fails, to prove Unmeasured.
type broken struct{}

func (broken) Files() []model.File {
	return []model.File{{Path: ".gitignore", Class: model.ClassTracked}}
}

func (broken) Read(string) ([]byte, error) { return nil, model.ErrUnreadable }

func (broken) Class(string) model.FileClass { return model.ClassTracked }

func TestGeneratedTracked(t *testing.T) {
	t.Parallel()
	res := scan(t, paths.GeneratedTracked(), map[string]string{
		".gitignore":            "bin/\n# gen/\n!gen/\ngenerated/\n gen \n**/sqlcgen/*\ntest/mocks/\napi/gen/**\n",
		"internal/x/.gitignore": "gen\n",
		"notes.md":              "gen/\n",
	})
	want := []string{".gitignore generated-ignored", ".gitignore generated-ignored", ".gitignore generated-ignored", ".gitignore generated-ignored", "internal/x/.gitignore generated-ignored"}
	if got := pathsOf(res); !slices.Equal(got, want) {
		t.Errorf("findings = %v, want %v", got, want)
	}
	lines := []int{res.Findings[0].Line, res.Findings[1].Line, res.Findings[2].Line, res.Findings[3].Line}
	if !slices.Equal(lines, []int{5, 6, 7, 8}) || res.Scope != "2 ignore files" {
		t.Errorf("lines = %v, scope %q", lines, res.Scope)
	}
	tree := broken{}
	unread, err := paths.GeneratedTracked().Run(t.Context(), model.Input{Mode: model.ModeScan, Policy: proof.Policy(), Snapshot: tree, Targets: tree.Files()})
	if err != nil || unread.Status.Kind != model.StatusUnmeasured || unread.Status.Reason != "read .gitignore: path unreadable" {
		t.Errorf("unreadable = %+v, %v", unread, err)
	}
}

func TestGeneratedTrackedReadsARuleFromItsFolder(t *testing.T) {
	t.Parallel()
	res := scan(t, paths.GeneratedTracked(), map[string]string{
		"test/.gitignore":     "mocks/\n",
		"api/test/.gitignore": "/mocks/**\n",
		"docs/.gitignore":     "mocks/\n",
		"test/x/.gitignore":   "mocks/\n",
	})
	want := []string{"api/test/.gitignore generated-ignored", "test/.gitignore generated-ignored"}
	if got := pathsOf(res); !slices.Equal(got, want) || res.Scope != "4 ignore files" {
		t.Errorf("findings = %v, scope %q, want %v", got, res.Scope, want)
	}
}

// unreadableIgnore is an ignore file the source could not read, with the
// class the source assigns to it.
type unreadableIgnore struct{}

func (unreadableIgnore) Files() []model.File {
	return []model.File{{Path: "api/.gitignore", Class: model.ClassUnreadable}}
}

func (unreadableIgnore) Read(string) ([]byte, error) { return nil, model.ErrUnreadable }

func (unreadableIgnore) Class(string) model.FileClass { return model.ClassUnreadable }

// TestGeneratedTrackedLeavesTheUnreadableClassUnmeasured reads an ignore
// file of the unreadable class through the snapshot, so the check answers
// Unmeasured naming it instead of clean (section 5.2).
func TestGeneratedTrackedLeavesTheUnreadableClassUnmeasured(t *testing.T) {
	t.Parallel()
	tree := unreadableIgnore{}
	res, err := paths.GeneratedTracked().Run(t.Context(), model.Input{Mode: model.ModeScan, Policy: proof.Policy(), Snapshot: tree, Targets: tree.Files()})
	if err != nil || res.Status.Kind != model.StatusUnmeasured || res.Status.Reason != "read api/.gitignore: path unreadable" {
		t.Errorf("unreadable ignore file = %+v, %v", res, err)
	}
}

// TestGeneratedTrackedLeavesAnIgnoreFileOverTheBoundUnread names an ignore
// file over the read bound among the files left unread and still judges
// the other ignore files.
func TestGeneratedTrackedLeavesAnIgnoreFileOverTheBoundUnread(t *testing.T) {
	t.Parallel()
	res := scan(t, paths.GeneratedTracked(), map[string]string{
		".gitignore":     strings.Repeat("x", model.MaxReadBytes+1),
		"api/.gitignore": "gen/\n",
	})
	if res.Status.Kind != model.StatusMeasured || !slices.Equal(res.Unread, []string{".gitignore"}) || !slices.Equal(pathsOf(res), []string{"api/.gitignore generated-ignored"}) {
		t.Fatalf("result = %+v", res)
	}
}
