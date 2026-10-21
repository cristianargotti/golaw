package setup

import "strings"

// runKey is the key of a command or job whose value the shell runs.
const runKey = "run"

// runValues returns the value of every run key in the text of a hook
// section, with its YAML quoting dropped. Only a run value reaches the
// shell, so fail_text, name, tags and every other key are left out, and a
// quoted scalar is skipped whole, which keeps a run: inside one from
// reading as a key.
func runValues(body string) []string {
	var values []string
	depth := 0
	for i := 0; i < len(body); i++ {
		c := body[i]
		switch {
		case !valueStart(body, i):
		case c == '"' || c == '\'':
			text, end := quotedScalar(body, i)
			if text == runKey && keyColon(body, end) {
				var value string
				value, end = runValue(body, end, column(body, i), depth > 0)
				values = append(values, value)
			}
			i = end - 1
		case c == '{' || c == '[':
			depth++
		case strings.HasPrefix(body[i:], runKey) && keyColon(body, i+len(runKey)):
			value, end := runValue(body, i+len(runKey), column(body, i), depth > 0)
			values = append(values, value)
			i = end - 1
		}
		if (c == '}' || c == ']') && depth > 0 {
			depth--
		}
	}
	return values
}

// valueStart reports whether a node may start at i: the first text of a
// line, the text after a flow indicator, or the text after a key or a list
// marker and a blank. An apostrophe inside a plain value such as don't
// starts nothing.
func valueStart(body string, i int) bool {
	for j := i - 1; j >= 0; j-- {
		switch body[j] {
		case ' ', '\t':
		case '\n', '{', '[', ',':
			return true
		case ':', '-', '?':
			return j < i-1
		default:
			return false
		}
	}
	return true
}

// column is the column of the text at i in its line.
func column(body string, i int) int { return i - strings.LastIndexByte(body[:i], '\n') - 1 }

// keyColon reports whether a key that ends at i is followed by the colon
// of a mapping: optional blanks, a colon, then a blank, a line end or a
// flow indicator.
func keyColon(body string, i int) bool {
	rest := strings.TrimLeft(body[i:], " \t")
	if !strings.HasPrefix(rest, ":") {
		return false
	}
	rest = rest[1:]
	return rest == "" || strings.ContainsRune(" \t\n,}]", rune(rest[0]))
}

// runValue reads the value of a run key that ends at key and sits at
// column, and returns it with the index after it.
func runValue(body string, key, column int, flow bool) (string, int) {
	i := key + strings.IndexByte(body[key:], ':') + 1
	i = skipBlanks(body, i)
	for i < len(body) && (body[i] == '&' || body[i] == '!') {
		i = skipBlanks(body, i+strings.IndexAny(body[i:]+" ", " \t\n"))
	}
	switch {
	case i < len(body) && (body[i] == '"' || body[i] == '\''):
		return quotedScalar(body, i)
	case flow:
		end := i + strings.IndexAny(body[i:]+"}", ",}]")
		return body[i:end], end
	}
	end := i + strings.IndexByte(body[i:]+"\n", '\n')
	head := body[i:end]
	if strings.HasPrefix(head, "|") || strings.HasPrefix(head, ">") {
		head = ""
	}
	more, next := indentedLines(body, end, column)
	return head + more, next
}

// skipBlanks returns the index of the first text at or after i that is no
// space or tab.
func skipBlanks(body string, i int) int { return len(body) - len(strings.TrimLeft(body[i:], " \t")) }

// indentedLines returns the lines after the line end at from that sit
// deeper than column, blank lines among them: the rest of a block scalar
// or of a plain scalar that spans lines.
func indentedLines(body string, from, column int) (string, int) {
	end := from
	for end < len(body) {
		line, _, _ := strings.Cut(body[end+1:], "\n")
		text := strings.TrimLeft(line, " \t")
		if text != "" && len(line)-len(text) <= column {
			break
		}
		end += 1 + len(line)
	}
	return body[from:end], end
}

// quotedScalar reads the YAML quoted scalar that starts at i, with its
// escapes resolved, and returns it with the index after its closing quote.
func quotedScalar(body string, i int) (string, int) {
	quote := body[i]
	var text []byte
	for j := i + 1; j < len(body); j++ {
		c := body[j]
		switch {
		case quote == '\'' && c == '\'' && strings.HasPrefix(body[j+1:], "'"):
			text = append(text, '\'')
			j++
		case c == quote:
			return string(text), j + 1
		case quote == '"' && c == '\\' && j+1 < len(body):
			j++
			text = append(text, escaped(body[j]))
		default:
			text = append(text, c)
		}
	}
	return string(text), len(body)
}

// escaped resolves the character after a backslash of a double-quoted
// scalar; a tab or a line feed reads as the blank that splits shell words.
func escaped(c byte) byte {
	if c == 't' || c == 'n' {
		return ' '
	}
	return c
}
