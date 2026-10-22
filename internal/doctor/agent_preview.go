package doctor

import (
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/hook"
)

// The lines of the Cursor compatibility preview (A4, D24).
const (
	idPreview             = "doctor.preview"
	idCapture             = "doctor.capture"
	codeWritesNotEnforced = "cursor-writes-not-enforced"
	codeCaptureRecorded   = "capture-recorded"
	codeCaptureAbsent     = "capture-absent"
	cursorPreTool         = "preToolUse"
)

// capturedTools are the preToolUse tools whose payload the Cursor adapter
// captures and allows until typed fixtures of the payload exist (A4).
func capturedTools() []string {
	return []string{"Write", "Delete"}
}

// previewLines say, for every installed Cursor, that it is a preview
// whose writes are not enforced.
func previewLines(f agentFacts) []Line {
	if f.Agent != agentCursor || f.Receipt.State != receiptRead {
		return nil
	}
	return []Line{{
		ID: idPreview, Key: f.scopeKey(), State: stateUnknown, Code: codeWritesNotEnforced,
		Detail: "Cursor writes: not enforced, payload unknown",
	}}
}

// captureLines say, per captured tool of an installed Cursor, which key
// names the last payload carried, the evidence the enforcement switch
// waits for; never a value.
func captureLines(f agentFacts) []Line {
	if f.Agent != agentCursor || f.Receipt.State != receiptRead {
		return nil
	}
	lines := make([]Line, 0, len(capturedTools()))
	for _, tool := range capturedTools() {
		line := Line{
			ID: idCapture, Key: agentCursor + "/" + cursorPreTool + "/" + tool, State: stateUnknown,
			Code: codeCaptureAbsent, Detail: "no " + tool + " payload was captured for this repository yet",
		}
		index := slices.IndexFunc(f.Captures, func(c hook.Capture) bool { return c.Event == cursorPreTool && c.Tool == tool })
		switch {
		case f.CaptureError != "":
			line.Detail = "the capture record cannot be read: " + f.CaptureError
		case index >= 0:
			line.Code = codeCaptureRecorded
			line.Detail = "the last " + tool + " payload carried the keys " + keysText(f.Captures[index].Keys)
		}
		lines = append(lines, line)
	}
	return lines
}

// previewSentence ends the sentence of a Cursor report, in the human
// channel, with what the preview means for the person (D24).
func previewSentence(portuguese bool) string {
	if portuguese {
		return "O Cursor é uma prévia de compatibilidade: as escritas dele ainda não são bloqueadas."
	}
	return "Cursor is a compatibility preview: its writes are not enforced yet."
}

// keysText lists key names, or says there were none.
func keysText(names []string) string {
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, ", ")
}
