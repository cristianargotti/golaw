package rung

import (
	pathpkg "path"
	"slices"
	"strconv"
	"strings"
)

// embedDirective starts the line that names the files a package embeds.
const embedDirective = "//go:embed"

// embedded reports whether a path is a file a package of the index may
// embed: a package directory above it holds a //go:embed pattern that
// names it or a directory it lies under. go reads such a file as an input
// of the package, so a worktree that differs there differs in a Go input.
// The patterns of each package are read once per call through patterns.
func (p *precommit) embedded(path string, dirs map[string]bool, patterns map[string][]string) bool {
	for dir := pathpkg.Dir(path); ; dir = pathpkg.Dir(dir) {
		if dirs[dir] {
			found, ok := patterns[dir]
			if !ok {
				found = p.embedPatterns(dir)
				patterns[dir] = found
			}
			rel := path
			if dir != "." {
				rel = strings.TrimPrefix(path, dir+"/")
			}
			if slices.ContainsFunc(found, func(pattern string) bool { return embedMatches(pattern, rel) }) {
				return true
			}
		}
		if dir == "." || dir == "/" {
			return false
		}
	}
}

// embedPatterns answers the patterns the Go files of a package directory
// of the index embed. A file the index cannot read answers a pattern that
// names every file, so the steps read the copy rather than guess.
func (p *precommit) embedPatterns(dir string) []string {
	var patterns []string
	files := p.scan.Index.Select(func(path string) bool {
		return pathpkg.Dir(path) == dir && strings.HasSuffix(path, goSuffix)
	})
	for _, file := range files {
		data, err := p.scan.Index.Read(file.Path)
		if err != nil {
			return []string{"*"}
		}
		if !strings.Contains(string(data), embedDirective) {
			continue
		}
		for line := range strings.SplitSeq(string(data), "\n") {
			if rest, ok := strings.CutPrefix(strings.TrimSpace(line), embedDirective); ok && (rest == "" || rest[0] == ' ' || rest[0] == '\t') {
				patterns = append(patterns, embedFields(rest)...)
			}
		}
	}
	return patterns
}

// embedFields splits the patterns of one directive as go reads them, a
// quoted pattern whole and unquoted, and drops the all: prefix, which only
// widens what a directory pattern holds.
func embedFields(rest string) []string {
	var out []string
	for rest = strings.TrimLeft(rest, " \t"); rest != ""; rest = strings.TrimLeft(rest, " \t") {
		field := rest[:quotedLen(rest)]
		rest = rest[len(field):]
		if unquoted, err := strconv.Unquote(field); err == nil {
			field = unquoted
		}
		out = append(out, strings.TrimPrefix(field, "all:"))
	}
	return out
}

// quotedLen answers the length of the first field of a directive: a quoted
// string through its closing quote, a backslash escaping the next byte of a
// double-quoted one, or the bytes up to the next space or tab.
func quotedLen(rest string) int {
	quote := rest[0]
	if quote != '"' && quote != '`' {
		if i := strings.IndexAny(rest, " \t"); i >= 0 {
			return i
		}
		return len(rest)
	}
	for i := 1; i < len(rest); i++ {
		switch {
		case rest[i] == '\\' && quote == '"':
			i++
		case rest[i] == quote:
			return i + 1
		}
	}
	return len(rest)
}

// embedMatches reports whether a pattern names the path, relative to the
// package directory, or a directory above it, whose whole tree a pattern
// that names a directory embeds. A pattern that does not parse names
// everything: go refuses it, and the copy shows the commit as it is.
func embedMatches(pattern, rel string) bool {
	parts := strings.Split(rel, "/")
	for i := 1; i <= len(parts); i++ {
		matched, err := pathpkg.Match(pattern, strings.Join(parts[:i], "/"))
		if err != nil || matched {
			return true
		}
	}
	return false
}
