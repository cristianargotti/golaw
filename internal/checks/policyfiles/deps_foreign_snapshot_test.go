package policyfiles_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestCodeownersForeignNoBaseOwnership(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		files   map[string][]byte
		changes []model.Change
		reason  string
	}{
		{
			name: "no staged changes does not read ownership", reason: "no staged change to judge",
			files: map[string][]byte{"CODEOWNERS": []byte("* unsupported\n")},
		},
		{
			name: "missing ownership", reason: "no CODEOWNERS file at the staged base",
			changes: []model.Change{{Op: model.ChangeAdd, NewPath: "private/a.txt"}},
		},
		{
			name: "only ownership added", reason: "no CODEOWNERS file at the staged base",
			files: map[string][]byte{"CODEOWNERS": []byte(foreignRules)},
			changes: []model.Change{
				{Op: model.ChangeAdd, NewPath: "CODEOWNERS"},
				{Op: model.ChangeAdd, NewPath: "private/a.txt"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := runForeign(t, proof.NewBlobTree(tc.files, nil), tc.changes)
			if result.Status.Kind != model.StatusNotApplicable || result.Status.Reason != tc.reason || len(result.Findings) != 0 {
				t.Fatalf("result = %+v", result)
			}
		})
	}
}

func TestCodeownersForeignUnreadableBaseNeverFinds(t *testing.T) {
	t.Parallel()
	files := map[string][]byte{"CODEOWNERS": []byte(foreignRules)}
	for _, tc := range []struct {
		name     string
		snapshot model.Snapshot
		before   string
		modified bool
		unread   bool
	}{
		{
			name: "snapshot read failure",
			snapshot: buildReadFailure{
				Snapshot: proof.NewBlobTree(files, nil), path: "CODEOWNERS", err: model.ErrUnreadable,
			},
		},
		{
			name: "snapshot read bound", unread: true,
			snapshot: buildReadFailure{
				Snapshot: proof.NewBlobTree(files, nil), path: "CODEOWNERS", err: model.ErrTooLarge,
			},
		},
		{name: "snapshot cannot read blobs", snapshot: proof.NewTree(files), before: "base", modified: true},
		{name: "base blob missing", snapshot: proof.NewBlobTree(files, nil), before: "base", modified: true},
		{name: "before id missing", snapshot: proof.NewBlobTree(files, nil), modified: true},
		{
			name: "base blob read bound", before: "base", modified: true, unread: true,
			snapshot: proof.NewBlobTree(files, map[string][]byte{"base": make([]byte, model.MaxReadBytes+1)}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			changes := []model.Change{{Op: model.ChangeAdd, NewPath: "private/a.txt"}}
			if tc.modified {
				changes = append(changes, model.Change{
					Op: model.ChangeModify, OldPath: "CODEOWNERS", NewPath: "CODEOWNERS", Before: tc.before,
				})
			}
			result := runForeign(t, tc.snapshot, changes)
			wantStatus := model.StatusUnmeasured
			var unread []string
			if tc.unread {
				wantStatus, unread = model.StatusMeasured, []string{"CODEOWNERS"}
			}
			if result.Status.Kind != wantStatus || len(result.Findings) != 0 || !slices.Equal(result.Unread, unread) {
				t.Fatalf("result = %+v", result)
			}
			if !tc.unread && !strings.Contains(result.Status.Reason, "CODEOWNERS") {
				t.Fatalf("reason does not name ownership evidence: %+v", result)
			}
		})
	}
}
