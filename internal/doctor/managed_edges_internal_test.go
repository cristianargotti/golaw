package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
)

// managedEdgeCase is one managed path, how its row lays it out under the
// root, and the error its evidence keeps, empty for a file doctor reads.
type managedEdgeCase struct {
	name, path, reason string
	arrange            func(t *testing.T, root, outside string)
}

// TestManagedLinesReadOnlyRegularFilesUnderTheRoot is the companion of
// TestManagedLinesNameEveryRecordedFile: init writes a managed file only as
// a regular file reached through directories, so a link on the way, which
// would hash bytes the repository does not hold, a device and a directory
// answer unknown without being read.
func TestManagedLinesReadOnlyRegularFilesUnderTheRoot(t *testing.T) {
	t.Parallel()
	for _, tc := range []managedEdgeCase{
		{"regular file", "golaw.mk", "", func(t *testing.T, root, _ string) {
			t.Helper()
			writeManagedFile(t, filepath.Join(root, "golaw.mk"))
		}},
		{"regular file in a directory", "tools/go.mod", "", func(t *testing.T, root, _ string) {
			t.Helper()
			writeManagedFile(t, filepath.Join(root, "tools", "go.mod"))
		}},
		{"linked file with the recorded bytes", "golaw.mk", "not a regular file", func(t *testing.T, root, outside string) {
			t.Helper()
			writeManagedFile(t, filepath.Join(outside, "golaw.mk"))
			linkManaged(t, filepath.Join(outside, "golaw.mk"), filepath.Join(root, "golaw.mk"))
		}},
		{"linked device", "golaw.mk", "not a regular file", func(t *testing.T, root, _ string) {
			t.Helper()
			linkManaged(t, os.DevNull, filepath.Join(root, "golaw.mk"))
		}},
		{"directory", "golaw.mk", "not a regular file", func(t *testing.T, root, _ string) {
			t.Helper()
			if err := os.Mkdir(filepath.Join(root, "golaw.mk"), 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{"linked directory on the way", "tools/go.mod", "tools is not a directory", func(t *testing.T, root, outside string) {
			t.Helper()
			writeManagedFile(t, filepath.Join(outside, "tools", "go.mod"))
			linkManaged(t, filepath.Join(outside, "tools"), filepath.Join(root, "tools"))
		}},
		{"file on the way", "tools/go.mod", "tools is not a directory", func(t *testing.T, root, _ string) {
			t.Helper()
			writeManagedFile(t, filepath.Join(root, "tools"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, outside := t.TempDir(), t.TempDir()
			tc.arrange(t, root, outside)
			requireManagedEdge(t, root, tc)
		})
	}
}

// requireManagedEdge requires a read file to match its record and any
// other entry to keep its reason, unread, as an unknown line.
func requireManagedEdge(t *testing.T, root string, tc managedEdgeCase) {
	t.Helper()
	facts := CollectManaged(root, map[string]string{tc.path: install.Digest([]byte(managedEdgeBytes))})
	lines := AssessManaged(facts)
	if len(facts) != 1 || len(lines) != 1 {
		t.Fatalf("one record gave facts %+v and lines %+v", facts, lines)
	}
	fact, line := facts[0], lines[0]
	if tc.reason == "" {
		if fact.Error != "" || fact.Missing || line.State != "ok" || line.Code != "managed-matches" {
			t.Fatalf("regular file = %+v, line %+v", fact, line)
		}
		return
	}
	if fact.Error != tc.reason || fact.Current != "" || fact.Missing ||
		line.State != "unknown" || line.Code != "managed-unreadable" || line.Detail != tc.path+": "+tc.reason {
		t.Fatalf("irregular entry = %+v, line %+v, want reason %q", fact, line, tc.reason)
	}
}

const managedEdgeBytes = "recorded\n"

// writeManagedFile writes the recorded bytes at path, creating its
// directory.
func writeManagedFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(managedEdgeBytes), 0o600); err != nil {
		t.Fatal(err)
	}
}

// linkManaged links path to target, and skips a row the platform cannot
// link, as Windows without the privilege to create links.
func linkManaged(t *testing.T, target, path string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
}
