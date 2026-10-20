package bootstrap_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestUpdateRestrictsWritesToRecordedPaths(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCurrent(t, filepath.Join(root, "known.txt"), "old\n")
	writeCurrent(t, filepath.Join(root, "unknown.txt"), "reviewed\n")
	oldHash := install.Digest([]byte("old\n"))
	recorded := []policy.ManagedFile{{Path: "unknown.txt", SHA256: oldHash}, {Path: "known.txt", SHA256: oldHash}}
	original := slices.Clone(recorded)
	plan, err := bootstrap.PlanUpdate(root, []assets.Rendered{
		templateFile("new.txt", "new\n", assets.OwnedByKit),
		templateFile("known.txt", "new\n", assets.OwnedByKit),
		templateFile("notes.txt", "notes\n", assets.OwnedByDeveloper),
	}, recorded)
	want := []policy.ManagedFile{
		{Path: "known.txt", SHA256: install.Digest([]byte("new\n"))},
		{Path: "unknown.txt", SHA256: oldHash},
	}
	if err != nil || len(plan.Changes) != 1 || plan.Changes[0].Path != "known.txt" ||
		!slices.Equal(plan.Managed, want) || len(plan.LocalEdits) != 0 || len(plan.Proposed) != 0 {
		t.Fatalf("restricted plan = %+v, %v", plan, err)
	}
	if !slices.Equal(recorded, original) {
		t.Fatalf("input records changed: %+v", recorded)
	}
	if applyErr := bootstrap.ApplyTemplates(root, plan, io.Discard); applyErr != nil {
		t.Fatal(applyErr)
	}
	requireUpdatedBytes(t, filepath.Join(root, "known.txt"), "new\n")
	requireUpdatedBytes(t, filepath.Join(root, "unknown.txt"), "reviewed\n")
	requireTemplateAbsent(t, root, "new.txt")
	requireTemplateAbsent(t, root, "notes.txt")
}

func TestUpdateRecordsExactHashesAndAbsence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, current string
		exists, local bool
	}{
		{"pristine", "old\n", true, false},
		{"accepted", "new\n", true, false},
		{"edited", "local\n", true, true},
		{"deleted", "", false, true},
		{"empty file", "", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path := filepath.Join(root, "config.txt")
			if tc.exists {
				if err := os.WriteFile(path, []byte(tc.current), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			oldHash, nextHash := install.Digest([]byte("old\n")), install.Digest([]byte("new\n"))
			plan, err := bootstrap.PlanUpdate(root, []assets.Rendered{
				templateFile("config.txt", "new\n", assets.OwnedByKit),
			}, []policy.ManagedFile{{Path: "config.txt", SHA256: oldHash}})
			if err != nil || len(plan.Managed) != 1 || len(plan.Proposed) != 0 {
				t.Fatalf("hash plan = %+v, %v", plan, err)
			}
			requireUpdateHashes(t, plan, tc.current, tc.exists, tc.local, oldHash, nextHash)
		})
	}
}

func requireUpdateHashes(t *testing.T, plan bootstrap.TemplatePlan, current string, exists, local bool, oldHash, nextHash string) {
	t.Helper()
	if !local {
		if len(plan.LocalEdits) != 0 || plan.Managed[0].SHA256 != nextHash {
			t.Fatalf("accepted baseline = %+v", plan)
		}
		return
	}
	currentHash := ""
	if exists {
		currentHash = install.Digest([]byte(current))
	}
	want := bootstrap.LocalEdit{Path: "config.txt", Recorded: oldHash, Current: currentHash, Proposed: nextHash}
	if len(plan.Changes) != 0 || len(plan.LocalEdits) != 1 || plan.LocalEdits[0] != want || plan.Managed[0].SHA256 != oldHash {
		t.Fatalf("local hash report = %+v, want %+v", plan, want)
	}
}

func TestUpdateRemovesLegacyOwnershipWithoutRecreatingFiles(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, current, proposal string
		ownership               assets.Ownership
	}{
		{"merged existing", "old\n", "old\nnew\n", assets.OwnedMerged},
		{"merged matching", "new\n", "", assets.OwnedMerged},
		{"merged deleted", "", "", assets.OwnedMerged},
		{"developer existing", "old\n", "", assets.OwnedByDeveloper},
		{"developer deleted", "", "", assets.OwnedByDeveloper},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path := filepath.Join(root, "config.txt")
			writeCurrent(t, path, tc.current)
			plan, err := bootstrap.PlanUpdate(root, []assets.Rendered{
				templateFile("config.txt", "new\n", tc.ownership),
			}, []policy.ManagedFile{{Path: "config.txt", SHA256: install.Digest([]byte("old\n"))}})
			if err != nil || len(plan.Managed) != 0 || len(plan.Changes) != 0 || len(plan.LocalEdits) != 0 {
				t.Fatalf("legacy ownership = %+v, %v", plan, err)
			}
			want := tc.current
			if tc.proposal != "" {
				want = tc.proposal
			}
			requireMergedTemplateProposal(t, plan, tc.current, want)
			var out bytes.Buffer
			if applyErr := bootstrap.ApplyTemplates(root, plan, &out); applyErr != nil {
				t.Fatal(applyErr)
			}
			requireUpdatedBytes(t, path, tc.current)
		})
	}
}

func TestUpdateDoesNotRecreateDeletedEmptyBaseline(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	emptyHash := install.Digest(nil)
	plan, err := bootstrap.PlanUpdate(root, []assets.Rendered{
		templateFile("empty.txt", "", assets.OwnedByKit),
	}, []policy.ManagedFile{{Path: "empty.txt", SHA256: emptyHash}})
	want := bootstrap.LocalEdit{Path: "empty.txt", Recorded: emptyHash, Proposed: emptyHash}
	if err != nil || len(plan.Changes) != 0 || len(plan.LocalEdits) != 1 || plan.LocalEdits[0] != want ||
		len(plan.Managed) != 1 || plan.Managed[0].SHA256 != emptyHash {
		t.Fatalf("deleted empty baseline = %+v, %v", plan, err)
	}
}
