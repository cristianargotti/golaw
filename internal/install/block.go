package install

import (
	"bytes"
	"fmt"
)

// The markers are matched by their bytes, not by whole lines, so a file
// saved with CRLF endings still shows its block and the carriage return
// after a marker stays a byte of the person's file.
const (
	blockBegin = "<!-- golaw:begin -->"
	blockEnd   = "<!-- golaw:end -->"
)

// agentBlockSpan answers the bytes of the marked block in current, from
// the first byte of the begin marker through the last byte of the end
// marker, without the line break that follows. A file without markers
// has no block; an orphan, reversed or repeated marker is refused,
// because replacing or removing around it could cut the person's text.
func agentBlockSpan(current []byte) (int, int, bool, error) {
	begins, ends := bytes.Count(current, []byte(blockBegin)), bytes.Count(current, []byte(blockEnd))
	if begins == 0 && ends == 0 {
		return 0, 0, false, nil
	}
	if begins != 1 || ends != 1 {
		return 0, 0, false, fmt.Errorf("agent block: %d begin and %d end markers: %w", begins, ends, ErrSettings)
	}
	start := bytes.Index(current, []byte(blockBegin))
	end := bytes.Index(current, []byte(blockEnd))
	if end < start+len(blockBegin) {
		return 0, 0, false, fmt.Errorf("agent block: end marker before begin marker: %w", ErrSettings)
	}
	return start, end + len(blockEnd), true, nil
}
