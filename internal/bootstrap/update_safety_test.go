package bootstrap_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestUpdateKeepsUnknownRecordsWithoutOpeningTheirPaths(t *testing.T) {
	t.Parallel()
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "unknown.txt")); err != nil {
		t.Fatal(err)
	}
	record := policy.ManagedFile{Path: "unknown.txt", SHA256: install.Digest([]byte("old\n"))}
	plan, err := bootstrap.PlanUpdate(root, nil, []policy.ManagedFile{record})
	if err != nil || len(plan.Managed) != 1 || plan.Managed[0] != record || len(plan.Changes) != 0 ||
		len(plan.Proposed) != 0 || len(plan.LocalEdits) != 0 {
		t.Fatalf("unknown record = %+v, %v", plan, err)
	}
	if applyErr := bootstrap.ApplyTemplates(root, plan, io.Discard); applyErr != nil {
		t.Fatal(applyErr)
	}
}

func TestUpdateCanonicalizesIdenticalRecordsAndRejectsConflictingBaselines(t *testing.T) {
	t.Parallel()
	for _, conflict := range []bool{false, true} {
		t.Run(map[bool]string{false: "identical", true: "conflicting"}[conflict], func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeCurrent(t, filepath.Join(root, "config.txt"), "old\n")
			record := policy.ManagedFile{Path: "config.txt", SHA256: install.Digest([]byte("old\n"))}
			recorded := []policy.ManagedFile{record, record}
			if conflict {
				recorded[1].SHA256 = install.Digest([]byte("other\n"))
			}
			plan, err := bootstrap.PlanUpdate(root, []assets.Rendered{
				templateFile("config.txt", "new\n", assets.OwnedByKit),
			}, recorded)
			requireDuplicateUpdateResult(t, plan, err, conflict)
		})
	}
}

func requireDuplicateUpdateResult(t *testing.T, plan bootstrap.TemplatePlan, err error, conflict bool) {
	t.Helper()
	if conflict {
		if !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), "config.txt") {
			t.Fatalf("conflicting records = %+v, %v", plan, err)
		}
		return
	}
	if err != nil || len(plan.Changes) != 1 || len(plan.Managed) != 1 {
		t.Fatalf("duplicate records = %+v, %v", plan, err)
	}
}

func TestUpdateRejectsUnsafeKnownPathsAndInvalidTemplates(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path string
		want       error
	}{
		{"traversal", "../escape.txt", install.ErrPath},
		{"policy", bootstrap.PolicyFile, install.ErrPath},
		{"link", "link.txt", bootstrap.ErrSymlink},
		{"duplicate template", "config.txt", install.ErrPath},
		{"unknown ownership", "config.txt", assets.ErrManifest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			rendered := []assets.Rendered{templateFile(tc.path, "new\n", assets.OwnedByKit)}
			switch tc.name {
			case "link":
				if err := os.Symlink(t.TempDir(), filepath.Join(root, tc.path)); err != nil {
					t.Fatal(err)
				}
			case "duplicate template":
				rendered = append(rendered, rendered[0])
			case "unknown ownership":
				rendered[0].Template.Ownership = "unknown"
			}
			_, err := bootstrap.PlanUpdate(root, rendered, []policy.ManagedFile{
				{Path: tc.path, SHA256: install.Digest([]byte("old\n"))},
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("invalid update = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestUpdateRechecksSnapshotsAndPrintsBeforeWrites(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"before output", "during output", "output fails", "policy changes"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path := filepath.Join(root, "config.txt")
			writeCurrent(t, path, "old\n")
			plan, err := bootstrap.PlanUpdate(root, []assets.Rendered{
				templateFile("config.txt", "new\n", assets.OwnedByKit),
			}, []policy.ManagedFile{{Path: "config.txt", SHA256: install.Digest([]byte("old\n"))}})
			if err != nil {
				t.Fatal(err)
			}
			writer, want := updateMutation(t, root, mode)
			if applyErr := bootstrap.ApplyTemplates(root, plan, writer); !errors.Is(applyErr, want) {
				t.Fatalf("changed update input = %v, want %v", applyErr, want)
			}
			current := "old\n"
			if mode == "before output" || mode == "during output" {
				current = "changed\n"
			}
			requireUpdatedBytes(t, path, current)
		})
	}
}

func updateMutation(t *testing.T, root, mode string) (io.Writer, error) {
	t.Helper()
	if mode == "output fails" {
		return &templateMutationWriter{fail: true}, io.ErrClosedPipe
	}
	mutate := func() { writeCurrent(t, filepath.Join(root, "config.txt"), "changed\n") }
	if mode == "policy changes" {
		mutate = func() {
			writeCurrent(t, filepath.Join(root, bootstrap.PolicyFile), "golaw: 1\nkit: {version: 0.1.0}\n")
		}
	}
	if mode == "before output" {
		mutate()
		return io.Discard, install.ErrChanged
	}
	return &templateMutationWriter{mutate: mutate}, install.ErrChanged
}
