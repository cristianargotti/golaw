package surface

import (
	"bytes"
	"strings"

	detect "github.com/cristianargotti/golaw/internal/surface"
)

// The marks of an HTML comment.
const (
	markupCommentOpen  = "<!--"
	markupCommentClose = "-->"
)

// element is one start tag of HTML markup: its name and attribute names
// lower-cased, the attribute values as written, and the line of its
// less-than sign.
type element struct {
	tag   string
	attrs map[string]string
	line  int
}

// markupScan walks the markup of one file, keeping the line of its
// position as it goes so no element recounts the file.
type markupScan struct {
	src  []byte
	pos  int
	line int
	seen int
}

// markupElements answers the start tags of the markup outside comments and
// outside the body of every script and style region. It never fails: an
// unterminated tag or comment runs to the end.
func markupElements(src []byte, regions []detect.Region) []element {
	s := &markupScan{src: src, line: 1}
	var out []element
	next := 0
	for s.pos < len(src) {
		if next < len(regions) && s.pos >= regions[next].Start {
			s.pos = max(s.pos, regions[next].End)
			next++
			continue
		}
		switch {
		case bytes.HasPrefix(src[s.pos:], []byte(markupCommentOpen)):
			s.skipComment()
		case src[s.pos] == '<' && s.pos+1 < len(src) && isLetter(src[s.pos+1]):
			out = append(out, s.startTag())
		default:
			s.pos++
		}
	}
	return out
}

// lineAt answers the line of the position, counting only the bytes since
// the last call, which the scan makes in order.
func (s *markupScan) lineAt() int {
	s.line += bytes.Count(s.src[s.seen:s.pos], []byte("\n"))
	s.seen = s.pos
	return s.line
}

// skipComment moves past the comment at the position.
func (s *markupScan) skipComment() {
	end := bytes.Index(s.src[s.pos+len(markupCommentOpen):], []byte(markupCommentClose))
	if end < 0 {
		s.pos = len(s.src)
		return
	}
	s.pos += len(markupCommentOpen) + end + len(markupCommentClose)
}

// startTag reads the tag at the position up to its closing sign.
func (s *markupScan) startTag() element {
	e := element{line: s.lineAt(), attrs: map[string]string{}}
	s.pos++
	e.tag = strings.ToLower(s.name())
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch {
		case c == '>':
			s.pos++
			return e
		case markupNameByte(c):
			name, value := s.attribute()
			e.attrs[name] = value
		default:
			s.pos++
		}
	}
	return e
}

// attribute reads one attribute: its name lower-cased and its value, quoted
// with either quote or unquoted up to a blank or the closing sign.
func (s *markupScan) attribute() (string, string) {
	name := strings.ToLower(s.name())
	s.blanks()
	if s.pos >= len(s.src) || s.src[s.pos] != '=' {
		return name, ""
	}
	s.pos++
	s.blanks()
	if s.pos < len(s.src) && (s.src[s.pos] == '"' || s.src[s.pos] == '\'') {
		quote := s.src[s.pos]
		start := s.pos + 1
		end := bytes.IndexByte(s.src[start:], quote)
		if end < 0 {
			s.pos = len(s.src)
			return name, string(s.src[start:])
		}
		s.pos = start + end + 1
		return name, string(s.src[start : start+end])
	}
	start := s.pos
	for s.pos < len(s.src) && !isBlank(s.src[s.pos]) && s.src[s.pos] != '>' {
		s.pos++
	}
	return name, string(s.src[start:s.pos])
}

// name reads a run of name bytes at the position.
func (s *markupScan) name() string {
	start := s.pos
	for s.pos < len(s.src) && markupNameByte(s.src[s.pos]) {
		s.pos++
	}
	return string(s.src[start:s.pos])
}

// blanks moves past blanks and line breaks.
func (s *markupScan) blanks() {
	for s.pos < len(s.src) && isBlank(s.src[s.pos]) {
		s.pos++
	}
}

func markupNameByte(c byte) bool {
	return isLetter(c) || isDigit(c) || c == '-' || c == ':' || c == '_'
}

func isBlank(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }
