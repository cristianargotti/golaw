package codex

import (
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// updating is an Update File hunk while its lines are read, with the added
// lines of its chunks apart, because the chunk shape mixes context and
// added lines in New.
type updating struct {
	hunk  Hunk
	added []string
}

// read takes one line of the hunk in the order the pinned parser tests it:
// after a chunk anchored at the end only a blank line or a new chunk, then
// the Move to line before any chunk, the chunk markers, the end anchor and
// the change lines. It answers the construct expected when the line is
// none of them, empty when the line was taken.
func (u *updating) read(line, marker string) string {
	if last := u.last(); last != nil && last.EOF {
		if marker == "" {
			return ""
		}
		if !isChunkMarker(marker) {
			return chunkMarker + " to open a chunk after " + eofMarker
		}
	}
	if to, ok := strings.CutPrefix(marker, moveMarker); ok && len(u.hunk.Chunks) == 0 && u.hunk.To == "" {
		u.hunk.Op, u.hunk.To = model.CandidateMove, to
		return ""
	}
	if isChunkMarker(marker) {
		return u.open(marker)
	}
	if marker == eofMarker {
		return u.anchor()
	}
	return u.change(line)
}

// isChunkMarker tells the bare @@ and the @@ followed by a context line.
func isChunkMarker(marker string) bool {
	return marker == chunkMarker || strings.HasPrefix(marker, chunkMarker+" ")
}

// open starts a chunk at its marker, with the context line after the
// marker when it has one; the chunk before it must hold a change line.
func (u *updating) open(marker string) string {
	if last := u.last(); last != nil && last.empty() {
		return "a change line in the chunk before " + chunkMarker
	}
	var chunk Chunk
	if marker != chunkMarker {
		chunk.Context = strings.TrimPrefix(marker, chunkMarker+" ")
	}
	u.hunk.Chunks = append(u.hunk.Chunks, chunk)
	return ""
}

// anchor marks the last chunk as anchored at the end of the file. The host
// takes an anchor before any chunk without effect.
func (u *updating) anchor() string {
	last := u.last()
	switch {
	case last == nil:
		return ""
	case last.empty():
		return "a change line before " + eofMarker
	default:
		last.EOF = true
		return ""
	}
}

// change takes a change line into the last chunk, opening the first chunk
// when the hunk has none, because the first chunk may omit its marker.
func (u *updating) change(line string) string {
	if line != "" && !strings.ContainsRune(" +-", rune(line[0])) {
		return "a change line, " + chunkMarker + " or a hunk header"
	}
	if len(u.hunk.Chunks) == 0 {
		u.hunk.Chunks = append(u.hunk.Chunks, Chunk{})
	}
	u.last().take(line)
	if added, ok := strings.CutPrefix(line, "+"); ok {
		u.added = append(u.added, added)
	}
	return ""
}

// complete refuses a hunk the host refuses at its end: one without a chunk
// and one whose last chunk holds no change line.
func (u *updating) complete() string {
	switch last := u.last(); {
	case last == nil:
		return "a chunk of changes in the " + strings.TrimSpace(updateMarker) + " hunk"
	case last.empty():
		return "a change line in the chunk"
	default:
		return ""
	}
}

// last is the chunk lines go to, none before the first chunk.
func (u *updating) last() *Chunk {
	if len(u.hunk.Chunks) == 0 {
		return nil
	}
	return &u.hunk.Chunks[len(u.hunk.Chunks)-1]
}

// take reads one change line into the chunk: an empty line is an empty
// context line, a space opens a context line, a plus an added line and a
// minus a removed line.
func (c *Chunk) take(line string) {
	switch {
	case line == "":
		c.Old, c.New = append(c.Old, ""), append(c.New, "")
	case line[0] == ' ':
		c.Old, c.New = append(c.Old, line[1:]), append(c.New, line[1:])
	case line[0] == '+':
		c.New = append(c.New, line[1:])
	default:
		c.Old = append(c.Old, line[1:])
	}
}

// empty reports a chunk without a change line.
func (c *Chunk) empty() bool { return len(c.Old) == 0 && len(c.New) == 0 }
