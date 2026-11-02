package dup

import (
	"fmt"
	"maps"
	"slices"
)

const (
	// defaultTokens and defaultLines are the bounds of A50, Sonar's own
	// minimum for a duplicated block of Go.
	defaultTokens = 100
	defaultLines  = 10
	percent       = 100
)

// Options bounds the smallest token sequence that counts as a copy.
type Options struct {
	MinTokens int
	MinLines  int
}

// Default answers the bounds of A50: 100 tokens over 10 lines.
func Default() Options {
	return Options{MinTokens: defaultTokens, MinLines: defaultLines}
}

// Span is one occurrence of a block: 1-based lines, inclusive.
type Span struct {
	Path       string
	Start, End int
}

// Block is one token sequence found two or more times; its spans are
// sorted by path, then by start line.
type Block struct {
	Tokens int
	Spans  []Span
}

// Report counts the new lines measured, test files included, and those
// inside a duplicated block.
type Report struct {
	Duplicated int
	New        int
	Blocks     []Block
}

// Percent answers the share of new lines inside a block; 0 when no line is
// new, so an empty change never reads as fully duplicated.
func (r Report) Percent() float64 {
	if r.New == 0 {
		return 0
	}
	return percent * float64(r.Duplicated) / float64(r.New)
}

// Measure tokenizes the Go files with go/scanner (comments dropped,
// literals normalized to their kind, identifiers kept), finds every token
// sequence of at least MinTokens spanning at least MinLines lines that
// occurs twice or more across the files, and counts the new lines (newLines
// nil means every line is new) that lie inside one.
func Measure(files map[string][]byte, newLines map[string][]int, o Options) (Report, error) {
	if o.MinTokens < 1 || o.MinLines < 1 {
		return Report{}, fmt.Errorf("%w: %d tokens, %d lines", ErrOptions, o.MinTokens, o.MinLines)
	}
	streams, err := tokenizeAll(files)
	if err != nil {
		return Report{}, err
	}
	blocks := findBlocks(streams, o)
	marked := markSpans(blocks)
	report := Report{Blocks: blocks}
	for index := range streams {
		duplicated, fresh := countLines(&streams[index], newLines, marked[streams[index].path])
		report.Duplicated += duplicated
		report.New += fresh
	}
	return report, nil
}

// markSpans answers, per path, every line inside a span, so members that
// overlap in one file count once.
func markSpans(blocks []Block) map[string]map[int]bool {
	marked := map[string]map[int]bool{}
	for _, block := range blocks {
		for _, span := range block.Spans {
			if marked[span.Path] == nil {
				marked[span.Path] = map[int]bool{}
			}
			for line := span.Start; line <= span.End; line++ {
				marked[span.Path][line] = true
			}
		}
	}
	return marked
}

// countLines counts only the lines holding a token, the way Sonar counts
// new lines of code, and the marked ones among them.
func countLines(s *stream, newLines map[string][]int, marked map[int]bool) (int, int) {
	duplicated, fresh := 0, 0
	tokenLines := map[int]bool{}
	for _, lexeme := range s.lexemes {
		tokenLines[lexeme.line] = true
	}
	lines := slices.Clone(newLines[s.path])
	if newLines == nil {
		lines = slices.Collect(maps.Keys(tokenLines))
	}
	slices.Sort(lines)
	for _, line := range slices.Compact(lines) {
		if !tokenLines[line] {
			continue
		}
		fresh++
		if marked[line] {
			duplicated++
		}
	}
	return duplicated, fresh
}
