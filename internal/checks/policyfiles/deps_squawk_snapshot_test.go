package policyfiles_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

type squawkSnapshot struct {
	*proof.Tree

	err      error
	reads    []string
	selected bool
	listed   bool
}

func (s *squawkSnapshot) Read(file string) ([]byte, error) {
	s.reads = append(s.reads, file)
	if s.err != nil {
		return nil, s.err
	}
	data, err := s.Tree.Read(file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", file, err)
	}
	return data, nil
}

func (s *squawkSnapshot) Files() []model.File {
	s.listed = true
	return s.Tree.Files()
}

func (s *squawkSnapshot) Select(match func(string) bool) []model.File {
	s.selected = true
	return model.Select(s.Tree, match)
}

func (s *squawkSnapshot) Any(match func(string) bool) bool { return model.Any(s.Tree, match) }

func TestSquawkConfigSelectsMigrationsWithoutReadingThem(t *testing.T) {
	t.Parallel()
	snapshot := &squawkSnapshot{Tree: proof.NewTree(map[string][]byte{
		".squawk.toml": []byte("pg_version = '17'\nassume_in_transaction = true\n" +
			"excluded_paths = ['db/migrations/0099_custom.sql']\n"),
		"db/migrations/0099_custom.sql": []byte("not parsed by this check"),
	})}
	got, err := policyfiles.SquawkConfig().Run(t.Context(), model.Input{Snapshot: snapshot})
	if err != nil || got.Status.Kind != model.StatusMeasured || len(got.Findings) != 1 {
		t.Fatalf("result = %+v, error = %v", got, err)
	}
	if snapshot.listed || !snapshot.selected || !slices.Equal(snapshot.reads, []string{".squawk.toml"}) {
		t.Fatalf("snapshot access = %+v", snapshot)
	}
}

func TestSquawkConfigSnapshotReadFailures(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		err    error
		status model.StatusKind
		unread []string
	}{
		{model.ErrUnreadable, model.StatusUnmeasured, nil},
		{model.ErrTooLarge, model.StatusMeasured, []string{".squawk.toml"}},
	} {
		t.Run(row.err.Error(), func(t *testing.T) {
			t.Parallel()
			snapshot := &squawkSnapshot{Tree: proof.NewTree(nil), err: row.err}
			got, err := policyfiles.SquawkConfig().Run(t.Context(), model.Input{Snapshot: snapshot})
			if err != nil || got.Status.Kind != row.status || len(got.Findings) != 0 ||
				!slices.Equal(got.Unread, row.unread) {
				t.Fatalf("result = %+v, error = %v", got, err)
			}
			if row.status == model.StatusUnmeasured && !strings.Contains(got.Status.Reason, row.err.Error()) {
				t.Fatalf("status = %+v, want read failure", got.Status)
			}
		})
	}
}
