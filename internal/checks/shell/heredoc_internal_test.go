package shell

import (
	"slices"
	"strings"
	"testing"
)

// TestUnendedBodiesAreReadOnce keeps data out of the command stream and
// prevents repeated body storage from growing with the square of its lines.
func TestUnendedBodiesAreReadOnce(t *testing.T) {
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
	if cats != 1 {
		t.Errorf("%d lines read as commands, want 1", cats)
	}
	if string(s.line) != "cat <<a\n" {
		t.Errorf("body lines entered the command stream: %d runes", len(s.line))
	}
	if !slices.Equal(s.bodies, []string{strings.Repeat("cat <<a\n", lines-1)}) {
		t.Errorf("the body was not kept exactly once: %q", s.bodies)
	}
}
