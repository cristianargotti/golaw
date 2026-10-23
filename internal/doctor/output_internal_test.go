package doctor

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

func TestJSONOutputAndHumanOutput(t *testing.T) {
	t.Parallel()
	report := assess(baseline(t))
	var first, second bytes.Buffer
	if err := Print(&first, report, true); err != nil {
		t.Fatal(err)
	}
	if err := WriteJSON(&second, report); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("JSON output is not deterministic")
	}
	var decoded Report
	if err := json.Unmarshal(first.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Schema != 1 || len(decoded.Lines) != len(report.Lines) {
		t.Fatal("JSON envelope lost diagnostics")
	}
	first.Reset()
	if err := Print(&first, report, false); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(first.Bytes(), []byte("doctor\n")) {
		t.Fatal("doctor output lacks its English heading")
	}
	for _, detail := range []string{"version=v0.1.0 sum=h1:fixture revision=revision", "installed=0.1.0 pinned=0.1.0"} {
		if !bytes.Contains(first.Bytes(), []byte(detail)) {
			t.Fatalf("text output lacks %q:\n%s", detail, first.String())
		}
	}
	if err := WriteJSON(closedWriter{}, report); err == nil {
		t.Fatal("JSON write failure ignored")
	}
	if err := WriteText(closedWriter{}, report); err == nil {
		t.Fatal("text write failure ignored")
	}
}

type closedWriter struct{}

func (closedWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}
