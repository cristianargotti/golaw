package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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

// TestManagedDriftNamesLineEndingsAlone covers a checkout that converts
// line endings, as core.autocrlf does on Windows without eol=lf: the LF
// file init wrote reads back with CRLF, golaw init --update keeps it as a
// local edit, so the line names that cause and the commands that write the
// file back with LF.
func TestManagedDriftNamesLineEndingsAlone(t *testing.T) {
	t.Parallel()
	edited := "golaw.mk differs from the hash init recorded: golaw init --update shows the diff and keeps the edit"
	endings := "golaw.mk differs from the hash init recorded only in its CRLF line endings, so golaw init --update keeps it as a local edit: " +
		"add the line * text=auto eol=lf to .gitattributes, run git rm --cached -q -- golaw.mk, " +
		"then git restore --source=HEAD --staged --worktree -- golaw.mk"
	for _, tc := range []struct {
		name, content, detail string
	}{
		{"converted line endings", "recorded\r\nsecond\r\n", endings},
		{"one converted line", "recorded\r\nsecond\n", endings},
		{"converted line endings and an edit", "recorded\r\nedited\r\n", edited},
		{"a lone carriage return", "recorded\rsecond\n", edited},
		{"an edit", "recorded\nedited\n", edited},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "golaw.mk"), []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}
			lines := replayedManaged(t, CollectManaged(root, map[string]string{
				"golaw.mk": install.Digest([]byte("recorded\nsecond\n")),
			}))
			if line := lines[0]; line.State != "drift" || line.Code != "managed-drift" || line.Detail != tc.detail {
				t.Fatalf("line = %+v, want detail %q", line, tc.detail)
			}
		})
	}
}

// replayedManaged assesses the evidence of one file, and requires the same
// assessment of the evidence after a JSON round trip.
func replayedManaged(t *testing.T, facts []ManagedFact) []Line {
	t.Helper()
	encoded, err := json.Marshal(facts)
	if err != nil {
		t.Fatal(err)
	}
	var replay []ManagedFact
	if err := json.Unmarshal(encoded, &replay); err != nil {
		t.Fatal(err)
	}
	lines := AssessManaged(replay)
	if !reflect.DeepEqual(lines, AssessManaged(facts)) || len(lines) != 1 {
		t.Fatalf("serialized evidence changed the assessment: %+v", lines)
	}
	return lines
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
