package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// The sizes of the profile scanner: a block line is short, a profile of a
// large module is long.
const (
	initialLine = 64 * 1024
	maximumLine = 1024 * 1024
)

// blockFields is the field count of a block line: range, statements, count.
const blockFields = 3

// fileCoverage accumulates the statements of one file. Blocks are keyed by
// their range so that a block reported by several test binaries counts
// once, covered when any run executed it.
type fileCoverage struct {
	blocks map[string]block
}

// block is one range of statements and whether any run executed it.
type block struct {
	stmts   int
	covered bool
}

// tally counts statements and covered statements.
type tally struct {
	stmts   int
	covered int
}

// percent is the covered share; an empty tally is zero.
func (t tally) percent() float64 {
	if t.stmts == 0 {
		return 0
	}
	return float64(t.covered) * 100 / float64(t.stmts)
}

// totals sums the statements and the covered statements of the file.
func (fc *fileCoverage) totals() tally {
	var t tally
	for _, b := range fc.blocks {
		t.stmts += b.stmts
		if b.covered {
			t.covered += b.stmts
		}
	}
	return t
}

// parseProfile reads the "mode:" header and the block lines
// "<file>:<start>,<end> <statements> <count>".
func parseProfile(r io.Reader) (map[string]*fileCoverage, error) {
	files := make(map[string]*fileCoverage)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, initialLine), maximumLine)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "mode:") {
			continue
		}
		if err := addBlock(files, text); err != nil {
			return nil, fmt.Errorf("profile line %d: %w", line, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read profile: %w", err)
	}
	return files, nil
}

// addBlock folds one block line into the file it belongs to.
func addBlock(files map[string]*fileCoverage, text string) error {
	fields := strings.Fields(text)
	if len(fields) != blockFields {
		return fmt.Errorf("%w: expected %d fields, got %d", ErrProfile, blockFields, len(fields))
	}
	name, span, ok := strings.Cut(fields[0], ":")
	if !ok {
		return fmt.Errorf("%w: missing range in %q", ErrProfile, fields[0])
	}
	stmts, err := strconv.Atoi(fields[1])
	if err != nil {
		return fmt.Errorf("%w: statements: %w", ErrProfile, err)
	}
	count, err := strconv.Atoi(fields[2])
	if err != nil {
		return fmt.Errorf("%w: count: %w", ErrProfile, err)
	}
	fc := files[name]
	if fc == nil {
		fc = &fileCoverage{blocks: make(map[string]block)}
		files[name] = fc
	}
	b := fc.blocks[span]
	b.stmts = stmts
	b.covered = b.covered || count > 0
	fc.blocks[span] = b
	return nil
}
