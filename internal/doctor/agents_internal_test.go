package doctor

import (
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// ids are the line ids of a report in order.
func ids(report Report) []string {
	out := make([]string, 0, len(report.Lines))
	for _, line := range report.Lines {
		out = append(out, line.ID)
	}
	return out
}

func TestClaudeReportKeepsTheM1LinesAndEndsWithItsDispatch(t *testing.T) {
	t.Parallel()
	input := Input{Root: t.TempDir(), ReceiptPath: "absent.json", Agent: agentClaude, Scope: "user"}
	store := &enrollmentStore{}
	m1 := Assess(Collect(t.Context(), input, store, NativeRunner{}))
	got := Claude(t.Context(), input, store)
	var kept []Line
	for _, line := range got.Lines {
		if !slices.Contains([]string{idInstalled, idTrusted, idFailMode, idState}, line.ID) {
			kept = append(kept, line)
		}
	}
	if !slices.Equal(kept, m1.Lines) {
		t.Fatalf("M1 lines = %v, want %v", ids(Report{Lines: kept}), ids(m1))
	}
	order := ids(got)
	if order[len(order)-1] != idDispatch || order[len(order)-2] != idState || !slices.Contains(order, idInstalled) || !slices.Contains(order, idTrusted) {
		t.Fatalf("order = %v", order)
	}
	if state := lineWith(got.Lines, idState); state.Code != codeNotInstalled || !strings.Contains(got.Summary, "rode golaw agent install --agent claude.") {
		t.Fatalf("state = %+v, summary = %q", state, got.Summary)
	}
}

func TestCodexAndCursorReportTheirFrame(t *testing.T) {
	t.Parallel()
	absent := filepath.Join(t.TempDir(), "receipt.json")
	for agent, report := range map[string]func(Input) Report{
		agentCodex:  func(in Input) Report { return Codex(t.Context(), in, &enrollmentStore{}) },
		agentCursor: func(in Input) Report { return Cursor(t.Context(), in, &enrollmentStore{}) },
	} {
		got := report(Input{Agent: agent, Scope: "user", ReceiptPath: absent})
		order := ids(got)
		if got.Schema != 1 || order[0] != idInstalled || order[len(order)-1] != idDispatch || lineWith(got.Lines, idState).Code != codeNotInstalled {
			t.Errorf("%s: %+v", agent, got)
		}
		if slices.Contains(order, "doctor.agent") || !strings.Contains(got.Summary, "golaw agent install --agent "+agent) {
			t.Errorf("%s: %v, %q", agent, order, got.Summary)
		}
	}
	in := installCursor(t)
	in.input.Dispatch, in.input.Language = Dispatch{State: DispatchObserved, OS: "linux"}, "en"
	got := Cursor(t.Context(), in.input, &enrollmentStore{})
	if state := lineWith(got.Lines, idState); state.State != stateUnknown || state.Code != codePreviewActive || !strings.HasPrefix(got.Summary, "golaw: the cursor hook is installed and its shell and read guards ran") {
		t.Fatalf("installed cursor = %+v, %q", got.Lines, got.Summary)
	}
}

func TestTheSummaryIsTextNeverJSON(t *testing.T) {
	t.Parallel()
	report := Report{Schema: 1, Lines: []Line{{ID: idState, Key: "codex/user", State: stateOK, Code: codeProtecting}}, Summary: "golaw: resumo."}
	var text, data bytes.Buffer
	if err := Print(&text, report, false); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(text.String(), "doctor.state codex/user: ok [protecting]\ngolaw: resumo.\n") {
		t.Fatalf("text = %q", text.String())
	}
	if err := Print(&data, report, true); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data.Bytes(), &decoded); err != nil || len(decoded) != 2 || strings.Contains(data.String(), "resumo") {
		t.Fatalf("json = %s, %v", data.String(), err)
	}
	if err := WriteText(&failingWriter{left: 2}, report); err == nil {
		t.Fatal("a failed summary write was ignored")
	}
}

// failingWriter accepts a number of writes and fails the next one.
type failingWriter struct{ left int }

func (w *failingWriter) Write(data []byte) (int, error) {
	if w.left == 0 {
		return 0, io.ErrClosedPipe
	}
	w.left--
	return len(data), nil
}
