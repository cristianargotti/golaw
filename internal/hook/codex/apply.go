package codex

import "slices"

// ApplyChunks applies the chunks of an Update File to the content of its
// file, in order, with a forward cursor over the original lines: the
// context line and the old lines of a chunk must match exactly once at or
// after the previous chunk, a chunk anchored at the end must match there,
// an empty old side appends at the end and an empty new side removes the
// match. It answers false, never a part of the file, when a chunk does not
// apply, so the caller judges the added lines alone (D3, D10); Codex then
// applies the patch with its own whitespace tolerance.
func ApplyChunks(content []byte, chunks []Chunk) ([]byte, bool) {
	if len(chunks) == 0 {
		return content, true
	}
	lines, ending, ok := splitLines(content)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(lines))
	cursor := 0
	for _, chunk := range chunks {
		start, found := locate(lines, cursor, chunk)
		if !found {
			return nil, false
		}
		out = append(append(out, lines[cursor:start]...), chunk.New...)
		cursor = start + len(chunk.Old)
	}
	return joinLines(append(out, lines[cursor:]...), ending), true
}

// locate is where the old lines of a chunk start in the original lines, at
// or after the cursor: after its context line when it has one, at the end
// when it is anchored there or has no old line, else at the one place its
// old lines match.
func locate(lines []string, cursor int, chunk Chunk) (int, bool) {
	from := cursor
	if chunk.Context != "" {
		at, found := once(lines, from, []string{chunk.Context})
		if !found {
			return 0, false
		}
		from = at + 1
	}
	if chunk.EOF || len(chunk.Old) == 0 {
		start := len(lines) - len(chunk.Old)
		if start < from {
			return 0, false
		}
		return start, slices.Equal(lines[start:], chunk.Old)
	}
	return once(lines, from, chunk.Old)
}

// once is the one index at or after from where the pattern matches the
// lines exactly; none or several is no match, because an ambiguous match
// would reconstruct a file Codex may write otherwise.
func once(lines []string, from int, pattern []string) (int, bool) {
	at, count := 0, 0
	for i := from; i+len(pattern) <= len(lines); i++ {
		if slices.Equal(lines[i:i+len(pattern)], pattern) {
			at, count = i, count+1
		}
	}
	return at, count == 1
}
