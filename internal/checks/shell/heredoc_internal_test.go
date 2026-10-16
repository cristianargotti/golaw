package shell

import (
	"slices"
	"strings"
	"testing"
)

// TestUnendedBodiesAreReadAgainOnce bounds the reading of a line whose
// here-documents never end: the lines after the first one that opens are
// read again as commands once each, opening no document of their own, so
// the bodies the reader keeps stay within the line instead of growing with
// the square of its lines, and every line is still a command.
func TestUnendedBodiesAreReadAgainOnce(t *testing.T) {
	t.Parallel()
	const lines = 400
	command := strings.Repeat("cat <<a\n", lines)
	s := scan(command, posix)
	kept := 0
	for _, body := range s.bodies {
		kept += len(body)
	}
	if kept > len(command) {
		t.Errorf("the bodies kept hold %d bytes for a line of %d", kept, len(command))
	}
	cats := 0
	for _, seg := range s.out {
		if slices.Equal(seg, segment{"cat"}) {
			cats++
		}
	}
	if cats != lines {
		t.Errorf("%d lines read as commands, want %d", cats, lines)
	}
}
