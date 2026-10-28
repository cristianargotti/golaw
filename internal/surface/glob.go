package surface

import (
	"fmt"
	"path"
	"strings"
)

// globAny is the segment that matches zero or more directories.
const globAny = "**"

// ValidPattern judges a pattern against the Sonar glob grammar golaw reads
// (M6.3.2): ** as a whole segment for zero or more directories, * for zero
// or more characters of one segment, ? for one, relative to the base
// directory, matched case-sensitively; a trailing slash means the whole
// directory. Braces, classes and escapes are refused rather than guessed,
// so the scope names them as Unmeasured.
func ValidPattern(pattern string) error {
	switch {
	case pattern == "":
		return fmt.Errorf("%w: the pattern is empty", ErrPattern)
	case strings.ContainsAny(pattern, `\[]{}`):
		return fmt.Errorf("%w: %q holds a character outside **, * and ?", ErrPattern, pattern)
	case strings.HasPrefix(pattern, "/"):
		return fmt.Errorf("%w: %q is not relative to the base directory", ErrPattern, pattern)
	}
	for _, segment := range segments(pattern) {
		switch {
		case segment == "" || segment == "." || segment == "..":
			return fmt.Errorf("%w: %q holds an empty, . or .. segment", ErrPattern, pattern)
		case segment != globAny && strings.Contains(segment, globAny):
			return fmt.Errorf("%w: %q uses ** inside a segment", ErrPattern, pattern)
		}
	}
	return nil
}

// Match reports whether a slash path matches a pattern ValidPattern
// accepts, and the refusal of one it does not.
func Match(pattern, name string) (bool, error) {
	if err := ValidPattern(pattern); err != nil {
		return false, err
	}
	return matchSegments(segments(pattern), strings.Split(name, "/")), nil
}

// segments splits a pattern, a trailing slash read as the whole directory.
func segments(pattern string) []string {
	if trimmed, directory := strings.CutSuffix(pattern, "/"); directory {
		pattern = trimmed + "/" + globAny
	}
	return strings.Split(pattern, "/")
}

// matchSegments matches segment by segment; path.Match judges one segment,
// and ValidPattern already refused the grammar path.Match could fail on.
func matchSegments(pattern, name []string) bool {
	for len(pattern) > 0 {
		if pattern[0] == globAny {
			return matchAnyDepth(pattern[1:], name)
		}
		if len(name) == 0 {
			return false
		}
		if ok, err := path.Match(pattern[0], name[0]); err != nil || !ok {
			return false
		}
		pattern, name = pattern[1:], name[1:]
	}
	return len(name) == 0
}

// matchAnyDepth matches the rest of a pattern after ** at every depth.
func matchAnyDepth(rest, name []string) bool {
	for skip := 0; skip <= len(name); skip++ {
		if matchSegments(rest, name[skip:]) {
			return true
		}
	}
	return false
}
