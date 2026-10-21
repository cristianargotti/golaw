package policyfiles

import (
	"fmt"
	"strings"
)

type makeBlock struct {
	inherited string
	openers   []string
}

const makeDefine = "define"

// wordElse is the one spelling of else that the make directives, the shell
// keywords of a demand line and the JSON Schema keywords share.
const wordElse = "else"

func (b *makeBlock) condition() string {
	if len(b.openers) != 0 {
		return b.openers[len(b.openers)-1]
	}
	return b.inherited
}

func (b *makeBlock) directive(line makeLine) bool {
	words := makeWords(line.text)
	if len(words) == 0 {
		return false
	}
	switch words[0] {
	case "ifeq", "ifneq", "ifdef", "ifndef":
		b.openers = append(b.openers, fmt.Sprintf("%s:%d", line.file, line.line))
	case wordElse:
		// Both branches remain conditional because no condition is evaluated.
	case "endif":
		if len(b.openers) != 0 {
			b.openers = b.openers[:len(b.openers)-1]
		}
	default:
		return false
	}
	return true
}

func (r *makeReader) define(lines []makeLine, index *int, condition string) bool {
	line := lines[*index]
	words := makeWords(strings.TrimSpace(line.text))
	if len(words) < 2 || words[0] != makeDefine {
		return false
	}
	var body []string
	depth := 1
	for *index++; *index < len(lines); *index++ {
		text := lines[*index].text
		trimmed := strings.TrimSpace(text)
		if !strings.HasPrefix(text, "\t") {
			depth += makeDefineDepth(trimmed)
		}
		if depth == 0 {
			break
		}
		body = append(body, text)
	}
	r.recordVar(makeVar{
		name: words[1], value: strings.Join(body, "\n"),
		file: line.file, line: line.line, condition: condition,
	}, makeDefine)
	return true
}

func makeDefineDepth(text string) int {
	if strings.HasPrefix(text, "define ") || strings.HasPrefix(text, "define\t") {
		return 1
	}
	if text == "endef" {
		return -1
	}
	return 0
}
