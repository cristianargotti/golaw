package surface

import (
	"bytes"
	"strings"
)

// hasPrefixFold reports an ASCII case-insensitive prefix; the markup names
// are ASCII, so the offsets of the content never move.
func hasPrefixFold(content []byte, prefix string) bool {
	return len(content) >= len(prefix) && strings.EqualFold(string(content[:len(prefix)]), prefix)
}

// tagAt reports an opening tag of the name at i: the name ends where the
// tag name ends, so <scripts> or <styled> is not one.
func tagAt(content []byte, i int, name string) bool {
	rest := content[i+1:]
	if !hasPrefixFold(rest, name) {
		return false
	}
	return len(rest) == len(name) || nameEnds(rest[len(name)])
}

// nameEnds reports the bytes that end a tag name.
func nameEnds(b byte) bool {
	return b == '>' || b == '/' || b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}

// openTag reads the attributes of an opening tag from the offset after its
// name, the names lower cased, a quoted value read whole so a > inside it
// does not end the tag; it answers them and the offset after the >.
func openTag(content []byte, i int) (map[string]string, int) {
	attributes := map[string]string{}
	for i < len(content) {
		switch b := content[i]; {
		case b == '>':
			return attributes, i + 1
		case nameEnds(b):
			i++
		default:
			var name, value string
			name, value, i = attribute(content, i)
			attributes[name] = value
		}
	}
	return attributes, len(content)
}

// attribute reads one attribute at i: its name, its value and the offset
// after it.
func attribute(content []byte, i int) (string, string, int) {
	start := i
	for i < len(content) && !nameEnds(content[i]) && content[i] != '=' {
		i++
	}
	name := strings.ToLower(string(content[start:i]))
	if i >= len(content) || content[i] != '=' {
		return name, "", i
	}
	i++
	if i < len(content) && (content[i] == '"' || content[i] == '\'') {
		quote := content[i]
		end := bytes.IndexByte(content[i+1:], quote)
		if end < 0 {
			return name, string(content[i+1:]), len(content)
		}
		return name, string(content[i+1 : i+1+end]), i + end + 2
	}
	start = i
	for i < len(content) && !nameEnds(content[i]) {
		i++
	}
	return name, string(content[start:i]), i
}

// closeTag answers the offset of the closing tag of the name from i, the
// end of the content when it never closes.
func closeTag(content []byte, i int, name string) int {
	for i < len(content) {
		next := bytes.Index(content[i:], []byte("</"))
		if next < 0 {
			return len(content)
		}
		i += next
		if tagAt(content, i+1, name) {
			return i
		}
		i += 2
	}
	return len(content)
}

// javaScriptType reports a type attribute that makes a script classic
// JavaScript or a module, the essence list of the HTML standard; a value
// with parameters is not a match, so a browser runs no such script either.
func javaScriptType(kind string) bool {
	switch kind {
	case "", "module", "text/javascript", "application/javascript", "text/ecmascript", "application/ecmascript",
		"application/x-javascript", "application/x-ecmascript", "text/x-javascript", "text/x-ecmascript",
		"text/jscript", "text/livescript", "text/javascript1.0", "text/javascript1.1", "text/javascript1.2",
		"text/javascript1.3", "text/javascript1.4", "text/javascript1.5":
		return true
	}
	return false
}
