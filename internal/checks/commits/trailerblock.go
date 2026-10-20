package commits

import "strings"

// The lines git writes itself into a trailer block, assembled from halves
// so this file carries neither: a paragraph they lead is a trailer block
// once a quarter of its lines are trailers.
const (
	signedOff     = "Sign" + "ed-off-by: "
	cherryPicked  = "(cherry picked from commit "
	dividerPrefix = "---"
)

// gitTrailers answers the lines git interpret-trailers reads as trailers,
// by index with their key: the lines with a key in the last paragraph after
// the subject, above a divider line and above the comment lines, the blank
// lines and the old Conflicts block that end the message. The paragraph is
// a trailer block when each of its lines is a trailer or the continuation
// of one, or when a line git writes itself is among them and a quarter of
// its lines are trailers. A key whose colon opens a link (https://) is
// prose, so a body that ends with a link is never read as a trailer.
func gitTrailers(ls []string, comment string) map[int]string {
	end := logEnd(ls, comment)
	keys := map[int]string{}
	for i := blockStart(ls[:end], comment); i < end; i++ {
		if key, ok := trailerKey(ls[i]); ok {
			keys[i] = key
		}
	}
	return keys
}

// logEnd is where git stops reading: the first divider line, less the
// lines that end the log.
func logEnd(ls []string, comment string) int {
	end := len(ls)
	for i, line := range ls {
		if divider(line) {
			end = i
			break
		}
	}
	return end - ignoredTail(ls[:end], comment)
}

// divider reports a line of three hyphens followed by white space or by
// nothing, above which git reads the log and below which a patch begins.
func divider(line string) bool {
	rest, ok := strings.CutPrefix(line, dividerPrefix)
	return ok && (rest == "" || strings.ContainsRune(" \t\r", rune(rest[0])))
}

// ignoredTail counts the lines git leaves out at the end of the log: a run
// of comment lines, empty lines and an old Conflicts block with its tab
// indented paths.
func ignoredTail(ls []string, comment string) int {
	first, conflicts := -1, false
	for i, line := range ls {
		switch {
		case line == "" || strings.HasPrefix(line, comment):
		case line == "Conflicts:":
			conflicts = true
		case conflicts && strings.HasPrefix(line, "\t"):
			continue
		default:
			first, conflicts = -1, false
			continue
		}
		if first < 0 {
			first = i
		}
	}
	if first < 0 {
		return 0
	}
	return len(ls) - first
}

// blockStart answers where the trailer block of the lines starts, past
// their end when they hold none: git reads the paragraphs after the
// subject from the last line up to the blank line above the last one.
func blockStart(ls []string, comment string) int {
	var c tally
	for i, title := len(ls)-1, subjectEnd(ls, comment); i >= title; i-- {
		switch line := ls[i]; {
		case strings.HasPrefix(line, comment):
			c.nonTrailers += c.continued
			c.continued = 0
		case blankLine(line) && !c.seen:
			// The blank lines below the last paragraph end nothing.
		case blankLine(line):
			c.nonTrailers += c.continued
			if c.block() {
				return i + 1
			}
			return len(ls)
		default:
			c.seen = true
			c.read(line)
		}
	}
	return len(ls)
}

// subjectEnd answers the first blank line that is not a comment, which
// ends the subject paragraph git never reads as trailers.
func subjectEnd(ls []string, comment string) int {
	for i, line := range ls {
		if !strings.HasPrefix(line, comment) && blankLine(line) {
			return i
		}
	}
	return len(ls)
}

// tally counts the lines of the last paragraph the way git does.
type tally struct {
	seen        bool // a line that is not blank was read
	recognized  bool // a line git writes itself was read
	trailers    int
	nonTrailers int
	continued   int // lines that continue a trailer when one is above them
}

func (c *tally) read(line string) {
	_, keyed := trailerKey(line)
	switch {
	case strings.HasPrefix(line, signedOff) || strings.HasPrefix(line, cherryPicked):
		c.trailers, c.continued, c.recognized = c.trailers+1, 0, true
	case keyed:
		c.trailers, c.continued = c.trailers+1, 0
	case line[0] == ' ' || line[0] == '\t':
		c.continued++
	default:
		c.nonTrailers, c.continued = c.nonTrailers+1+c.continued, 0
	}
}

// block reports whether the paragraph is a trailer block.
func (c *tally) block() bool {
	return (c.recognized && c.trailers*3 >= c.nonTrailers) || (c.trailers > 0 && c.nonTrailers == 0)
}

// trailerKey answers the key of a trailer line as git finds its separator:
// letters, digits and hyphens from the first byte, white space only after
// them, then a colon that does not open a link.
func trailerKey(line string) (string, bool) {
	spaced := false
	for i := range len(line) {
		switch c := line[i]; {
		case c == ':':
			return strings.TrimRight(line[:i], " \t"), i > 0 && !strings.HasPrefix(line[i+1:], "//")
		case !spaced && (c == '-' || isAlnum(c)):
		case i > 0 && (c == ' ' || c == '\t'):
			spaced = true
		default:
			return "", false
		}
	}
	return "", false
}

func isAlnum(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9'
}

func blankLine(line string) bool { return strings.Trim(line, gitSpace) == "" }
