package codex

import (
	"fmt"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// The markers of the apply_patch grammar pinned to PatchGrammar (D10,
// survey section 2.3), compared on a trimmed line as Codex compares them.
const (
	beginMarker       = "*** Begin Patch"
	endMarker         = "*** End Patch"
	environmentMarker = "*** Environment ID:"
	addMarker         = "*** Add File: "
	deleteMarker      = "*** Delete File: "
	updateMarker      = "*** Update File: "
	moveMarker        = "*** Move to: "
	eofMarker         = "*** End of File"
	chunkMarker       = "@@"
	headerPrefix      = "***"
	heredocEnd        = "EOF"
	// heredocLines is the least a here-document wrapped patch holds: the two
	// wrapper lines and the two markers.
	heredocLines = 4
	// The number of the first body line in the text: after the begin
	// marker, or after the wrapper line and the begin marker.
	plainFirst   = 2
	wrappedFirst = 3
)

// Hunk is one file of a patch: what the patch does to it, the path of its
// header as written, the Move to path when it has one, the content lines
// of an Add File and the chunks of an Update File.
type Hunk struct {
	Op     model.CandidateOp
	Path   string
	To     string
	Lines  []string
	Chunks []Chunk
}

// Chunk is one change of an Update File: the text after its @@ marker, the
// context and removed lines, the context and added lines, and whether it
// is anchored at the end of the file.
type Chunk struct {
	Context string
	Old     []string
	New     []string
	EOF     bool
}

// patchDoc is a parsed patch: the environment its Environment ID line
// selects, empty when the line is absent, and its hunks.
type patchDoc struct {
	Environment string
	Hunks       []Hunk
}

// ParsePatch reads the apply_patch grammar of PatchGrammar: a text that
// does not parse is cannot judge, named by its line and the construct
// expected there, never by its content.
func ParsePatch(text string) ([]Hunk, error) {
	doc, err := parsePatch(text)
	if err != nil {
		return nil, err
	}
	return doc.Hunks, nil
}

// parsePatch is ParsePatch with the environment the text selects kept,
// because a patch of another environment applies where the hook cannot
// read.
func parsePatch(text string) (patchDoc, error) {
	doc, _, err := parseDoc(text)
	return doc, err
}

// parseDoc reads a patch once: its environment line, its hunks and, apart,
// the added lines of each hunk, so the plus provenance of a line survives
// the chunk shape that mixes context and added lines in New.
func parseDoc(text string) (patchDoc, [][]string, error) {
	r, err := openPatch(text)
	if err != nil {
		return patchDoc{}, nil, err
	}
	var doc patchDoc
	if doc.Environment, err = r.environment(); err != nil {
		return patchDoc{}, nil, err
	}
	var added [][]string
	for !r.done() {
		if strings.TrimSpace(r.peek()) == endMarker {
			return doc, added, r.ended()
		}
		hunk, lines, hunkErr := r.hunk()
		if hunkErr != nil {
			return patchDoc{}, nil, hunkErr
		}
		doc.Hunks = append(doc.Hunks, hunk)
		added = append(added, lines)
	}
	return doc, added, nil
}

// reader walks the lines of a patch body: the index of the next line and
// the number the first line of the body has in the text, for the reasons.
type reader struct {
	lines []string
	at    int
	first int
}

// openPatch reads the body of a patch: the lines between its markers, with
// the here-document wrapper Codex accepts around them unwrapped first. The
// host splits the trimmed text into lines without the carriage return
// before each line feed, and its line reader drops one more from every
// line of the body, so a line keeps no carriage return the host drops.
func openPatch(text string) (*reader, error) {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSuffix(line, "\r")
	}
	if inside, ok := between(lines); ok {
		return &reader{lines: body(inside), first: plainFirst}, nil
	}
	last := lines[len(lines)-1]
	if len(lines) >= heredocLines && heredocStart(lines[0]) && strings.HasSuffix(last, heredocEnd) {
		if inside, ok := between(lines[1 : len(lines)-1]); ok {
			return &reader{lines: body(inside), first: wrappedFirst}, nil
		}
	}
	return nil, fmt.Errorf("%w: apply_patch: expected %s on the first line and %s on the last", model.ErrCannotJudge, beginMarker, endMarker)
}

// body is the lines of a patch body with the carriage return the line
// reader of the host drops removed from the end of each.
func body(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = strings.TrimSuffix(line, "\r")
	}
	return out
}

// between is the lines between the begin and end markers when the first
// and the last line are those markers.
func between(lines []string) ([]string, bool) {
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != beginMarker || strings.TrimSpace(lines[len(lines)-1]) != endMarker {
		return nil, false
	}
	return lines[1 : len(lines)-1], true
}

// heredocStart tells the three first lines of a wrapped patch apart from
// any other line, because the shell quoting of the delimiter is free; the
// host compares the line whole.
func heredocStart(line string) bool {
	switch line {
	case "<<EOF", "<<'EOF'", `<<"EOF"`:
		return true
	default:
		return false
	}
}

// environment reads the optional Environment ID line directly after the
// begin marker; a second one is a header no hunk has, which hunk refuses.
func (r *reader) environment() (string, error) {
	if r.done() {
		return "", nil
	}
	rest, ok := strings.CutPrefix(strings.TrimSpace(r.peek()), environmentMarker)
	if !ok {
		return "", nil
	}
	id := strings.TrimSpace(rest)
	if id == "" {
		return "", r.fail("an environment id after " + environmentMarker)
	}
	r.at++
	return id, nil
}

func (r *reader) done() bool { return r.at >= len(r.lines) }

func (r *reader) peek() string { return r.lines[r.at] }

// fail names the line and the construct expected there.
func (r *reader) fail(expected string) error {
	return fmt.Errorf("%w: apply_patch line %d: expected %s", model.ErrCannotJudge, r.first+r.at, expected)
}

// ended reads what follows an end marker before the last line: the host
// takes blank lines there and refuses anything else.
func (r *reader) ended() error {
	for r.at++; !r.done(); r.at++ {
		if strings.TrimSpace(r.peek()) != "" {
			return r.fail("only blank lines after " + endMarker)
		}
	}
	return nil
}
