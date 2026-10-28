package runnertest

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The fake consumes its input in the child process, which the cover
// profile of the parent never sees, so consume, appendLine and the
// decoding of Inputs are also proven in process.

// failingReader refuses every read, as a closed stdin of a broken parent.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.ErrUnsupported }

func TestConsumeRecordsTheInputAndWaitsTheDelay(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	base := filepath.Join(dir, "ruff")
	start := time.Now()
	if err := (Behavior{Stdin: true, Delay: "50ms"}).consume(base, strings.NewReader("x = 1\n")); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took < 50*time.Millisecond {
		t.Errorf("consume returned after %v, want at least the 50ms delay", took)
	}
	if got := Inputs(t, dir, "ruff"); len(got) != 1 || !bytes.Equal(got[0], []byte("x = 1\n")) {
		t.Errorf("Inputs = %q", got)
	}
}

func TestConsumeRefusesWhatItCannotRead(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tests := map[string]struct {
		behavior Behavior
		base     string
	}{
		"an unreadable delay":                  {behavior: Behavior{Delay: "soon"}, base: filepath.Join(dir, "a")},
		"a record in a directory that is gone": {behavior: Behavior{Stdin: true}, base: filepath.Join(dir, "absent", "b")},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := tc.behavior.consume(tc.base, strings.NewReader("")); err == nil {
				t.Error("consume = nil, want an error the fake turns into BrokenCode")
			}
		})
	}
	if err := (Behavior{Stdin: true}).consume(filepath.Join(dir, "c"), failingReader{}); err == nil {
		t.Error("a stdin that cannot be read is refused")
	}
	if err := (Behavior{}).consume(filepath.Join(dir, "d"), failingReader{}); err != nil {
		t.Errorf("a behavior that does not ask for stdin never reads it: %v", err)
	}
}

func TestInputsKeepsAnEmptyInputApartFromNone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	base := filepath.Join(dir, "shfmt")
	for _, input := range [][]byte{{}, []byte("a\nb\n")} {
		if err := appendLine(base+inputsSuffix, input); err != nil {
			t.Fatal(err)
		}
	}
	got := Inputs(t, dir, "shfmt")
	if len(got) != 2 || got[0] == nil || len(got[0]) != 0 || string(got[1]) != "a\nb\n" {
		t.Errorf("Inputs = %q", got)
	}
}
