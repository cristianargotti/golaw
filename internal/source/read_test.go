package source_test

import (
	"fmt"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

// mustRead reads a path through a snapshot and fails the test on an error.
func mustRead(t *testing.T, s model.Snapshot, rel string) string {
	t.Helper()
	data, err := s.Read(rel)
	if err != nil {
		t.Fatalf("Read(%s): %v", rel, err)
	}
	return string(data)
}

// readErr answers the error of one read, nil when it succeeded.
func readErr(s model.Snapshot, rel string) error {
	if _, err := s.Read(rel); err != nil {
		return fmt.Errorf("read %s: %w", rel, err)
	}
	return nil
}
