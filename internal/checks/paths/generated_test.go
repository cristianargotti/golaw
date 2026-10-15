package paths_test

import (
	"slices"
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
