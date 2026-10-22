package codex

import (
	"strings"
	"unicode"

	"github.com/cristianargotti/golaw/internal/model"
)

// hunk reads one hunk from its header: an Add File with its plus lines, a
// Delete File, or an Update File with an optional Move to and its chunks.
// It answers the hunk and its added lines apart.
func (r *reader) hunk() (Hunk, []string, error) {
	marker, path := splitHeader(strings.TrimSpace(r.peek()))
	if path == "" {
		return Hunk{}, nil, r.fail("a hunk header with a path: " + addMarker + ", " + deleteMarker + " or " + updateMarker)
	}
	r.at++
	switch marker {
	case addMarker:
		lines := r.added()
		return Hunk{Op: model.CandidateCreate, Path: path, Lines: lines}, lines, nil
	case deleteMarker:
		return Hunk{Op: model.CandidateDelete, Path: path}, nil, nil
	default:
		return r.update(path)
	}
}

// splitHeader tells the marker of a hunk header and the path after it,
// both empty when the line is no hunk header or names no path.
func splitHeader(line string) (string, string) {
	for _, marker := range []string{addMarker, deleteMarker, updateMarker} {
		if rest, ok := strings.CutPrefix(line, marker); ok && strings.TrimSpace(rest) != "" {
			return marker, rest
		}
	}
	return "", ""
}

// added reads the content of an Add File: every following line that starts
// with a plus, without it; the first other line ends the hunk, so a header
// right after the Add File header is an empty file.
func (r *reader) added() []string {
	var lines []string
	for !r.done() {
		content, ok := strings.CutPrefix(r.peek(), "+")
		if !ok {
			break
		}
		lines = append(lines, content)
		r.at++
	}
	return lines
}

// update reads an Update File hunk the way the pinned parser reads one:
// each line is tested with its trailing white space trimmed against the
// hunk headers and the end marker, which close the hunk, and then taken by
// the hunk; a hunk without a chunk, or whose last chunk holds no change
// line, is refused.
func (r *reader) update(path string) (Hunk, []string, error) {
	u := updating{hunk: Hunk{Op: model.CandidateEdit, Path: path}}
	for ; !r.done(); r.at++ {
		line := r.peek()
		marker := strings.TrimRightFunc(line, unicode.IsSpace)
		if marker == endMarker || isHeader(marker) {
			break
		}
		if expected := u.read(line, marker); expected != "" {
			return Hunk{}, nil, r.fail(expected)
		}
	}
	if expected := u.complete(); expected != "" {
		return Hunk{}, nil, r.fail(expected)
	}
	return u.hunk, u.added, nil
}

// isHeader tells a hunk header that names a path.
func isHeader(line string) bool {
	_, path := splitHeader(line)
	return path != ""
}
