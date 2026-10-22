package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/hook"
)

func readCaptureRecord(t *testing.T, path string) CaptureRecord {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record CaptureRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	return record
}

func TestSaveCaptureKeepsOneEntryPerEventAndTool(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "cursor", "captures", "identity.json")
	at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	write := hook.Capture{Event: "preToolUse", Tool: "Write", Keys: []string{"content", "file_path"}}
	remove := hook.Capture{Event: "preToolUse", Tool: "Delete", Keys: []string{"file_path"}}
	again := hook.Capture{Event: "preToolUse", Tool: "Write", Keys: []string{"contents", "path"}}
	for _, c := range []hook.Capture{write, remove, again} {
		record := CaptureRecord{Identity: "identity", HookDigest: "definition", Captures: []hook.Capture{c}, SeenAt: at}
		if err := SaveCapture(path, record); err != nil {
			t.Fatal(err)
		}
	}
	got := readCaptureRecord(t, path)
	if len(got.Captures) != 2 || got.Captures[0].Tool != "Delete" || !slices.Equal(got.Captures[1].Keys, again.Keys) {
		t.Fatalf("%+v", got)
	}
	other := CaptureRecord{Identity: "other", HookDigest: "definition", Captures: []hook.Capture{remove}, SeenAt: at}
	if err := SaveCapture(path, other); err != nil {
		t.Fatal(err)
	}
	if got = readCaptureRecord(t, path); got.Identity != "other" || len(got.Captures) != 1 {
		t.Fatalf("a record of another repository was merged: %+v", got)
	}
}

func TestSaveCaptureReplacesAnUnreadableRecord(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "identity.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	record := CaptureRecord{Identity: "identity", HookDigest: "definition", Captures: []hook.Capture{{Event: "preToolUse", Tool: "Write", Keys: []string{"a"}}}}
	if err := SaveCapture(path, record); err != nil {
		t.Fatal(err)
	}
	if got := readCaptureRecord(t, path); len(got.Captures) != 1 {
		t.Fatalf("%+v", got)
	}
	if err := SaveCapture(filepath.Join(path, "below-a-file.json"), record); err == nil {
		t.Fatal("a record under a file was written")
	}
}
