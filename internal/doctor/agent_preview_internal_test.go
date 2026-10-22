package doctor

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/hook"
)

func TestLoadCapturesReadsOnlyTheCurrentRepositoryAndDefinition(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "captures", "identity.json")
	write := hook.Capture{Event: cursorPreTool, Tool: "Write", Keys: []string{"content", "file_path"}}
	record := CaptureRecord{Identity: "identity", HookDigest: "digest", Captures: []hook.Capture{write}, SeenAt: time.Date(2026, time.October, 21, 9, 0, 0, 0, time.UTC)}
	if err := SaveCapture(path, record); err != nil {
		t.Fatal(err)
	}
	if got, err := LoadCaptures(path, "identity", "digest"); err != nil || !reflect.DeepEqual(got, []hook.Capture{write}) {
		t.Fatalf("captures = %+v, %v", got, err)
	}
	for name, args := range map[string][3]string{
		"another repository": {path, "other", "digest"},
		"another definition": {path, "identity", "other"},
		"no record":          {filepath.Join(dir, "absent.json"), "identity", "digest"},
	} {
		if got, err := LoadCaptures(args[0], args[1], args[2]); err != nil || got != nil {
			t.Errorf("%s: %+v, %v", name, got, err)
		}
	}
	broken := filepath.Join(dir, "broken.json")
	if err := os.WriteFile(broken, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCaptures(broken, "identity", "digest"); err == nil || !strings.Contains(err.Error(), "decode the capture record") {
		t.Fatalf("a broken record = %v", err)
	}
	if _, err := LoadCaptures(dir, "identity", "digest"); err == nil || !strings.Contains(err.Error(), "read the capture record") {
		t.Fatalf("a directory = %v", err)
	}
}

func TestPreviewAndCaptureLinesBelongToCursorAlone(t *testing.T) {
	t.Parallel()
	codex := installedFacts(agentCodex, "user")
	if previewLines(codex) != nil || captureLines(codex) != nil {
		t.Fatal("a Codex report carries Cursor lines")
	}
	cursor := installedFacts(agentCursor, "user")
	preview := previewLines(cursor)
	want := Line{ID: idPreview, Key: "cursor/user", State: stateUnknown, Code: codeWritesNotEnforced, Detail: "Cursor writes: not enforced, payload unknown"}
	if len(preview) != 1 || preview[0] != want {
		t.Fatalf("preview = %+v", preview)
	}
	cursor.Captures = []hook.Capture{{Event: cursorPreTool, Tool: "Write", Keys: []string{"content", "file_path"}}, {Event: cursorPreTool, Tool: "Delete"}}
	lines := captureLines(cursor)
	if len(lines) != 2 || lines[0].Key != "cursor/preToolUse/Write" || lines[0].Code != codeCaptureRecorded ||
		lines[0].Detail != "the last Write payload carried the keys content, file_path" || lines[1].Detail != "the last Delete payload carried the keys (none)" {
		t.Fatalf("recorded = %+v", lines)
	}
	cursor.Captures = nil
	if lines := captureLines(cursor); lines[0].Code != codeCaptureAbsent || lines[1].State != stateUnknown || !strings.Contains(lines[1].Detail, "no Delete payload") {
		t.Fatalf("absent = %+v", lines)
	}
	cursor.CaptureError = "decode the capture record: unexpected end of JSON input"
	if lines := captureLines(cursor); lines[0].Code != codeCaptureAbsent || !strings.Contains(lines[0].Detail, "cannot be read: decode") {
		t.Fatalf("unreadable = %+v", lines)
	}
	cursor.Receipt.State = receiptAbsent
	if previewLines(cursor) != nil || captureLines(cursor) != nil {
		t.Fatal("a Cursor that is not installed carries the preview lines")
	}
}

func TestCursorReportCarriesThePreviewAndTheCaptures(t *testing.T) {
	t.Parallel()
	in := installCursor(t)
	in.input.Captures = []hook.Capture{{Event: cursorPreTool, Tool: "Write", Keys: []string{"file_path"}}}
	got := Cursor(t.Context(), in.input, &enrollmentStore{})
	order := ids(got)
	if !slices.Contains(order, idPreview) || !slices.Contains(order, idCapture) || order[len(order)-1] != idDispatch {
		t.Fatalf("order = %v", order)
	}
	if captured := collectAgent(in.input); !reflect.DeepEqual(captured.Captures, in.input.Captures) {
		t.Fatalf("captures = %+v", captured.Captures)
	}
	if !strings.HasSuffix(got.Summary, " O Cursor é uma prévia de compatibilidade: as escritas dele ainda não são bloqueadas.") {
		t.Fatalf("summary = %q", got.Summary)
	}
	in.input.Language = "en"
	if got := Cursor(t.Context(), in.input, &enrollmentStore{}); !strings.HasSuffix(got.Summary, " Cursor is a compatibility preview: its writes are not enforced yet.") {
		t.Fatalf("summary = %q", got.Summary)
	}
	codex := Codex(t.Context(), Input{Agent: agentCodex, Scope: "user", ReceiptPath: in.input.ReceiptPath + ".absent"}, &enrollmentStore{})
	if strings.Contains(codex.Summary, "Cursor") {
		t.Fatalf("a Codex summary names the preview: %q", codex.Summary)
	}
}
