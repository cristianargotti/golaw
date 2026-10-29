package newcode

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"strconv"
	"strings"
	"unicode/utf16"

	"golang.org/x/mod/modfile"
)

// LineHits holds only the lines Sonar counts as lines to cover, each true
// when any block that reaches it ran.
type LineHits map[int]bool

const (
	profileFields = 3
	coordinates   = 4
	// A long generated path must not cut the report short.
	maxProfileLine = 1024 * 1024
)

type block struct {
	path                                       string
	startLine, startCol, endLine, endCol, hits int
}

func (b block) ordered() bool {
	return b.startLine >= 1 && b.startCol >= 1 && b.endLine >= b.startLine && b.endCol >= 1
}

type profileReader struct {
	module  string
	read    func(string) ([]byte, error)
	sources map[string][]string
	out     map[string]LineHits
}

// Lines reads a cover profile into the lines Sonar's Go importer credits,
// keyed by the module-relative slash path read answers the source of.
func Lines(profile io.Reader, module string, read func(path string) ([]byte, error)) (map[string]LineHits, error) {
	scanner := bufio.NewScanner(profile)
	scanner.Buffer(nil, maxProfileLine)
	if !scanner.Scan() || !knownMode(strings.TrimSuffix(scanner.Text(), "\r")) {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrReport, err)
		}
		return nil, fmt.Errorf("%w: the report is empty or opens without a mode line", ErrReport)
	}
	reader := &profileReader{module: module, read: read, sources: map[string][]string{}, out: map[string]LineHits{}}
	for number := 2; scanner.Scan(); number++ {
		text := strings.TrimSuffix(scanner.Text(), "\r")
		if text == "" {
			continue
		}
		if err := reader.add(text); err != nil {
			return nil, fmt.Errorf("report line %d: %w", number, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrReport, err)
	}
	return reader.out, nil
}

func knownMode(line string) bool {
	return line == "mode: set" || line == "mode: count" || line == "mode: atomic"
}

func parseBlock(text, module string) (block, error) {
	name, rest, cut := strings.CutLast(text, ":")
	if !cut {
		return block{}, noBlock(text)
	}
	path, found := strings.CutPrefix(name, module+"/")
	if !found || !fs.ValidPath(path) {
		return block{}, fmt.Errorf("%w: %s is outside the module %s", ErrReport, name, module)
	}
	fields := strings.Fields(rest)
	if len(fields) != profileFields {
		return block{}, noBlock(text)
	}
	item := block{path: path}
	spaced := strings.NewReplacer(".", " ", ",", " ").Replace(fields[0])
	scanned, err := fmt.Sscanf(spaced, "%d %d %d %d", &item.startLine, &item.startCol, &item.endLine, &item.endCol)
	if err != nil || scanned != coordinates {
		return block{}, noBlock(text)
	}
	statements, stmtErr := strconv.Atoi(fields[1])
	hits, hitErr := strconv.Atoi(fields[2])
	if stmtErr != nil || hitErr != nil || statements < 0 || hits < 0 || !item.ordered() {
		return block{}, noBlock(text)
	}
	item.hits = hits
	return item, nil
}

func noBlock(text string) error {
	return fmt.Errorf("%w: %q is no block", ErrReport, text)
}

func (r *profileReader) add(text string) error {
	item, err := parseBlock(text, r.module)
	if err != nil {
		return err
	}
	lines, seen := r.sources[item.path]
	if !seen {
		source, readErr := r.read(item.path)
		if readErr != nil {
			return fmt.Errorf("%w: %s, which the report names, cannot be read: %w", ErrReport, item.path, readErr)
		}
		lines = strings.Split(strings.TrimSuffix(string(source), "\n"), "\n")
		r.sources[item.path] = lines
	}
	if item.startLine > len(lines) {
		return fmt.Errorf("%w: %s has no line %d, the report is older than the source", ErrReport, item.path, item.startLine)
	}
	hits, named := r.out[item.path]
	if !named {
		hits = LineHits{}
		r.out[item.path] = hits
	}
	first, last := trimBlock(item, lines)
	for line := first; line <= last; line++ {
		if strings.TrimSpace(lines[line-1]) != "" {
			hits[line] = hits[line] || item.hits > 0
		}
	}
	return nil
}

// trimBlock answers the first and last line a block covers once its edges
// lose braces and control characters, because Sonar's Go importer trims
// them this way and does not weight statements, so a closing brace line is
// never a line to cover.
func trimBlock(item block, lines []string) (int, int) {
	startLine, startCol := item.startLine, item.startCol
	for ignored(lines, startLine, startCol) {
		startCol++
		if startCol > width(lines[startLine-1]) {
			startLine, startCol = startLine+1, 1
		}
	}
	endLine, endCol := item.endLine, item.endCol-1
	if endLine > len(lines) {
		endLine, endCol = len(lines), width(lines[len(lines)-1])
	}
	for after(endLine, endCol, startLine, startCol) && ignored(lines, endLine, endCol) {
		endCol--
		if endCol < 1 {
			endLine--
			endCol = width(lines[endLine-1])
		}
	}
	return startLine, endLine
}

func after(line, column, otherLine, otherColumn int) bool {
	return line > otherLine || line == otherLine && column > otherColumn
}

func width(line string) int {
	return len(utf16.Encode([]rune(line)))
}

func ignored(lines []string, line, column int) bool {
	if line < 1 || line > len(lines) || column < 1 {
		return false
	}
	units := utf16.Encode([]rune(lines[line-1]))
	if column > len(units) {
		return true
	}
	unit := units[column-1]
	return unit < ' ' || unit == '{' || unit == '}'
}

// Module answers the module path that prefixes every block of the profile.
func Module(gomod []byte) (string, error) {
	module := modfile.ModulePath(gomod)
	if module == "" {
		return "", ErrModule
	}
	return module, nil
}
