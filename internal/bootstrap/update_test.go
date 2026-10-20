package bootstrap_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
)

// updateCase is one managed file an update meets: its bytes now, the bytes
// it must hold after, its ownership class, and the changes and local edits
// the plan names.
type updateCase struct {
	name, current, want string
	ownership           assets.Ownership
	changes, edits      int
}

func TestUpdateUsesRecordedHashesAndPreservesLocalEdits(t *testing.T) {
	t.Parallel()
	for _, tc := range []updateCase{
		{"pristine", "old\n", "new\n", assets.OwnedByKit, 1, 0},
		{"local edit", "local\n", "local\n", assets.OwnedByKit, 0, 1},
		{"local deletion", "", "", assets.OwnedByKit, 0, 1},
		{"manually accepted", "new\n", "new\n", assets.OwnedByKit, 0, 0},
		{"developer file", "old\n", "old\n", assets.OwnedByDeveloper, 0, 0},
		{"merged file", "old\n", "old\n", assets.OwnedMerged, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path := filepath.Join(root, "config.txt")
			writeCurrent(t, path, tc.current)
			plan := requireUpdatePlan(t, root, tc)
			var out bytes.Buffer
			if err := bootstrap.ApplyTemplates(root, plan, &out); err != nil {
				t.Fatal(err)
			}
			requireUpdatedBytes(t, path, tc.want)
		})
	}
}

// writeCurrent writes the bytes the managed file of a row holds now, when
// it holds any.
func writeCurrent(t *testing.T, path, current string) {
	t.Helper()
	if current == "" {
		return
	}
	if err := os.WriteFile(path, []byte(current), 0o600); err != nil {
		t.Fatal(err)
	}
}

// requireUpdatePlan plans the update of a row against the hash init
// recorded and requires its changes, its local edits, the baseline a local
// edit keeps and no managed entry for a file a person owns.
func requireUpdatePlan(t *testing.T, root string, tc updateCase) bootstrap.TemplatePlan {
	t.Helper()
	oldHash := install.Digest([]byte("old\n"))
	nextHash := install.Digest([]byte("new\n"))
	files := []assets.Rendered{{
		Template: assets.Template{
			Source:      "templates/config.txt.tmpl",
			Destination: "config.txt", Ownership: tc.ownership,
		},
		Path: "config.txt", Data: []byte("new\n"), SHA256: nextHash,
	}}
	recorded := []policy.ManagedFile{{Path: "config.txt", SHA256: oldHash}}
	plan, err := bootstrap.PlanUpdate(root, files, recorded)
	if err != nil || len(plan.Changes) != tc.changes || len(plan.LocalEdits) != tc.edits {
		t.Fatalf("plan = %+v, %v", plan, err)
	}
	if tc.edits != 0 && plan.Managed[0].SHA256 != oldHash {
		t.Fatal("local edit replaced its recorded baseline")
	}
	if tc.ownership != assets.OwnedByKit && len(plan.Managed) != 0 {
		t.Fatal("person-owned file remained managed")
	}
	return plan
}

// requireUpdatedBytes requires the managed file to hold the bytes of the
// row, or a deleted file to stay deleted.
func requireUpdatedBytes(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if want == "" {
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("deleted file recreated: %v", err)
		}
		return
	}
	if err != nil || string(data) != want {
		t.Fatalf("updated bytes = %q, %v", data, err)
	}
}
