package dup

import (
	"cmp"
	"slices"
	"strings"
)

// keySeparator cannot occur inside a token key, so two different windows
// never join to the same key.
const keySeparator = "\x00"

type position struct {
	stream, start int
}

func comparePositions(a, b position) int {
	return cmp.Or(cmp.Compare(a.stream, b.stream), cmp.Compare(a.start, b.start))
}

// spans tells whether the window at start covers MinLines lines, so a copy
// packed on a few lines stays below the bound Sonar applies.
func (s *stream) spans(start int, o Options) bool {
	first := s.lexemes[start].line
	last := s.lexemes[start+o.MinTokens-1].line
	return last-first+1 >= o.MinLines
}

func (s *stream) windowKey(start, size int) string {
	keys := make([]string, 0, size)
	for _, lexeme := range s.lexemes[start : start+size] {
		keys = append(keys, lexeme.key)
	}
	return strings.Join(keys, keySeparator)
}

// groups keys every window of MinTokens lexemes that spans enough lines.
func groups(streams []stream, o Options) map[string][]position {
	out := map[string][]position{}
	for index := range streams {
		s := &streams[index]
		for start := 0; start+o.MinTokens <= len(s.lexemes); start++ {
			if !s.spans(start, o) {
				continue
			}
			key := s.windowKey(start, o.MinTokens)
			out[key] = appendMember(out[key], position{stream: index, start: start}, o.MinTokens)
		}
	}
	return out
}

// appendMember drops a window that overlaps the previous member of the same
// stream, because a repeated run inside one sequence is not a second copy.
func appendMember(members []position, p position, size int) []position {
	if n := len(members); n > 0 {
		last := members[n-1]
		if last.stream == p.stream && p.start < last.start+size {
			return members
		}
	}
	return append(members, p)
}

// findBlocks merges the consecutive windows that keep every member together
// into one block, so a long copy reports once with its full extent.
func findBlocks(streams []stream, o Options) []Block {
	all := groups(streams, o)
	keyAt := map[position]string{}
	repeated := make([][]position, 0, len(all))
	for key, members := range all {
		if len(members) < 2 {
			continue
		}
		for _, member := range members {
			keyAt[member] = key
		}
		repeated = append(repeated, members)
	}
	slices.SortFunc(repeated, func(a, b []position) int { return comparePositions(a[0], b[0]) })
	visited := map[position]bool{}
	var blocks []Block
	for _, members := range repeated {
		if visited[members[0]] {
			continue
		}
		extra := extend(members, keyAt, all)
		for _, member := range members {
			for step := 0; step <= extra; step++ {
				visited[position{stream: member.stream, start: member.start + step}] = true
			}
		}
		blocks = append(blocks, block(streams, members, o.MinTokens+extra))
	}
	return blocks
}

// extend counts the following windows in which every member still sits in
// one group of the same size.
func extend(members []position, keyAt map[position]string, all map[string][]position) int {
	for extra := 1; ; extra++ {
		key, ok := keyAt[position{stream: members[0].stream, start: members[0].start + extra}]
		if !ok || len(all[key]) != len(members) {
			return extra - 1
		}
		for _, member := range members[1:] {
			if keyAt[position{stream: member.stream, start: member.start + extra}] != key {
				return extra - 1
			}
		}
	}
}

func block(streams []stream, members []position, tokens int) Block {
	spans := make([]Span, 0, len(members))
	for _, member := range members {
		s := &streams[member.stream]
		spans = append(spans, Span{
			Path:  s.path,
			Start: s.lexemes[member.start].line,
			End:   s.lexemes[member.start+tokens-1].line,
		})
	}
	slices.SortFunc(spans, func(a, b Span) int {
		return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Start, b.Start))
	})
	return Block{Tokens: tokens, Spans: spans}
}
