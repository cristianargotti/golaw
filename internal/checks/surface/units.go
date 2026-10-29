package surface

import (
	"strings"

	detect "github.com/cristianargotti/golaw/internal/surface"
)

// unit is one stretch of script a check reads as a whole: a script file,
// or one inline script of an HTML file.
type unit struct {
	language detect.Language
	tokens   []token
}

// scriptUnits answers the script units of a file. Every inline script of
// an HTML file is one, spliced ones included, because a Go template action
// inside a script leaves the code around it as the browser runs it.
func scriptUnits(f source) []unit {
	if f.info.Language != detect.HTML {
		jsx := f.info.Language == detect.JSX || f.info.Language == detect.TSX
		return []unit{{language: f.info.Language, tokens: lexJS(f.src, 1, jsx)}}
	}
	var out []unit
	for _, r := range f.info.Regions {
		if r.Kind == detect.RegionScript {
			out = append(out, unit{language: detect.JavaScript, tokens: lexJS(f.src[r.Start:r.End], r.StartLine, false)})
		}
	}
	return out
}

// significant drops the comments and the JSX text, which hold no code.
func significant(tokens []token) []token {
	out := make([]token, 0, len(tokens))
	for _, t := range tokens {
		if t.kind != tokComment && t.kind != tokText {
			out = append(out, t)
		}
	}
	return out
}

// closer answers the bracket that closes an opening one, empty for any
// other text.
func closer(opener string) string {
	switch opener {
	case "(":
		return ")"
	case "[":
		return "]"
	case "{":
		return "}"
	}
	return ""
}

// closing answers the index of the bracket that closes the one at open in a
// significant stream, len(tokens) when it is never closed.
func closing(tokens []token, open int) int {
	opener := tokens[open].text
	closes := closer(opener)
	depth := 0
	for i := open; i < len(tokens); i++ {
		if tokens[i].kind != tokPunct {
			continue
		}
		switch tokens[i].text {
		case opener:
			depth++
		case closes:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(tokens)
}

// stringValue answers the text of a string or template token without its
// quotes, with the backslash before a quote or a backslash removed; an
// unterminated literal answers the text to its end.
func stringValue(t token) string {
	if t.text == "" {
		return ""
	}
	quote, text := t.text[0], t.text[1:]
	out := make([]byte, 0, len(text))
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == quote {
			break
		}
		if c == '\\' && i+1 < len(text) && strings.IndexByte(escapable, text[i+1]) >= 0 {
			i++
			c = text[i]
		}
		out = append(out, c)
	}
	return string(out)
}

// escapable are the bytes whose backslash stringValue removes.
const escapable = "\\'\"`"
