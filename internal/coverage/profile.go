package coverage

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
)

// Bound each line independently so a large profile need not be buffered.
const (
	initialLine = 64 * 1024
	maximumLine = 1024 * 1024
)

const blockFields = 3

// Blocks are keyed by their range so a block reported by several test binaries counts
// once, covered when any run executed it.
type fileCoverage struct {
	blocks map[string]block
}

type block struct {
	stmts   int
	covered bool
}

type tally struct {
	stmts   int
	covered int
}

func (fc *fileCoverage) totals() (tally, error) {
	var t tally
	for _, b := range fc.blocks {
		if b.stmts > math.MaxInt-t.stmts {
			return tally{}, fmt.Errorf("%w: file statement total overflows int", ErrProfile)
		}
		t.stmts += b.stmts
		if b.covered {
			t.covered += b.stmts
		}
	}
	return t, nil
}

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

func addBlock(files map[string]*fileCoverage, text string) error {
	location, counts, err := splitBlock(text)
	if err != nil {
		return err
	}
	name, span, err := profileLocation(location)
	if err != nil {
		return err
	}
	stmts, err := profileNumber(counts[0], "statements")
	if err != nil {
		return err
	}
	count, err := profileNumber(counts[1], "count")
	if err != nil {
		return err
	}
	return mergeBlock(files, name, span, stmts, count)
}

// splitBlock reads the two counts from the end of the line and keeps the
// text before them as the location: go test writes a source name as it is,
// so a name may hold a space.
func splitBlock(text string) (string, []string, error) {
	fields := strings.Fields(text)
	if len(fields) < blockFields {
		return "", nil, fmt.Errorf("%w: expected at least %d fields, got %d", ErrProfile, blockFields, len(fields))
	}
	counts := fields[len(fields)-blockFields+1:]
	location := strings.TrimSpace(text)
	for _, field := range slices.Backward(counts) {
		location = strings.TrimSpace(strings.TrimSuffix(location, field))
	}
	return location, counts, nil
}

func mergeBlock(files map[string]*fileCoverage, name, span string, stmts, count int) error {
	fc := files[name]
	if fc == nil {
		fc = &fileCoverage{blocks: make(map[string]block)}
		files[name] = fc
	}
	b, exists := fc.blocks[span]
	if exists && b.stmts != stmts {
		return fmt.Errorf("%w: contradictory statement counts for %s:%s", ErrProfile, name, span)
	}
	b.stmts = stmts
	b.covered = b.covered || count > 0
	fc.blocks[span] = b
	return nil
}

func profileLocation(location string) (string, string, error) {
	separator := strings.LastIndexByte(location, ':')
	if separator <= 0 || separator == len(location)-1 {
		return "", "", fmt.Errorf("%w: missing source or range in %q", ErrProfile, location)
	}
	start, end, found := strings.Cut(location[separator+1:], ",")
	if !found {
		return "", "", fmt.Errorf("%w: missing range end in %q", ErrProfile, location)
	}
	start, err := profilePosition(start)
	if err != nil {
		return "", "", err
	}
	end, err = profilePosition(end)
	if err != nil {
		return "", "", err
	}
	return location[:separator], start + "," + end, nil
}

func profilePosition(position string) (string, error) {
	line, column, found := strings.Cut(position, ".")
	if !found {
		return "", fmt.Errorf("%w: missing column in %q", ErrProfile, position)
	}
	lineNumber, err := profileNumber(line, "line")
	if err != nil {
		return "", err
	}
	columnNumber, err := profileNumber(column, "column")
	if err != nil {
		return "", err
	}
	return strconv.Itoa(lineNumber) + "." + strconv.Itoa(columnNumber), nil
}

func profileNumber(value, field string) (int, error) {
	number, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%w: %s: %w", ErrProfile, field, err)
	}
	if number < 0 {
		return 0, fmt.Errorf("%w: negative %s", ErrProfile, field)
	}
	return number, nil
}
