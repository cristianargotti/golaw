package policyfiles

import "strings"

type makeLine struct {
	text string
	file string
	line int
}

func makeLines(file string, data []byte) []makeLine {
	physical := strings.Split(string(data), "\n")
	lines := make([]makeLine, 0, len(physical))
	for index := 0; index < len(physical); index++ {
		line := makeLine{text: strings.TrimSuffix(physical[index], "\r"), file: file, line: index + 1}
		if strings.HasPrefix(line.text, "\t") {
			line.text = makeRecipeLine(physical, &index)
		} else {
			for makeContinued(line.text) && index+1 < len(physical) {
				index++
				line.text = strings.TrimRight(line.text[:len(line.text)-1], " \t") + " " +
					strings.TrimLeft(strings.TrimSuffix(physical[index], "\r"), " \t")
			}
			if comment := makeComment(line.text); comment >= 0 {
				line.text = line.text[:comment]
			}
		}
		lines = append(lines, line)
	}
	return lines
}

// makeRecipeLine keeps backslash-newline pairs in one recipe entry so their
// shell meaning and continuation comments survive without creating rules.
func makeRecipeLine(physical []string, index *int) string {
	text := strings.TrimSuffix(physical[*index], "\r")
	recipe := []string{text}
	for makeContinued(text) && *index+1 < len(physical) {
		*index++
		text = strings.TrimSuffix(physical[*index], "\r")
		recipe = append(recipe, strings.TrimPrefix(text, "\t"))
	}
	return strings.Join(recipe, "\n")
}

func makeContinued(text string) bool {
	backslashes := 0
	for index := len(text) - 1; index >= 0 && text[index] == '\\'; index-- {
		backslashes++
	}
	return backslashes%2 != 0
}

func makeComment(text string) int {
	for index := 0; index < len(text); index++ {
		switch text[index] {
		case '\\':
			index++
		case '#':
			return index
		}
	}
	return -1
}

func makeWords(text string) []string {
	const whitespace = " \t\r\n\v\f"
	var words []string
	for text = strings.TrimLeft(text, whitespace); text != ""; text = strings.TrimLeft(text, whitespace) {
		end := makeDelimiter(text, whitespace)
		if end < 0 {
			words = append(words, text)
			break
		}
		words = append(words, text[:end])
		text = text[end:]
	}
	return words
}

func makeDelimiter(text, delimiters string) int {
	for index := 0; index < len(text); index++ {
		switch {
		case text[index] == '\\':
			index++
		case makeReferenceStart(text, index):
			index = makeReferenceEnd(text, index)
		case strings.ContainsRune(delimiters, rune(text[index])):
			return index
		}
	}
	return -1
}

func makeReferenceStart(text string, index int) bool {
	return text[index] == '$' && index+1 < len(text) &&
		(text[index+1] == '(' || text[index+1] == '{')
}

func makeReferenceEnd(text string, start int) int {
	var stack []byte
	for index := start + 1; index < len(text); index++ {
		switch text[index] {
		case '(':
			stack = append(stack, ')')
		case '{':
			stack = append(stack, '}')
		case ')', '}':
			if len(stack) > 0 && text[index] == stack[len(stack)-1] {
				stack = stack[:len(stack)-1]
			}
		}
		if len(stack) == 0 {
			return index
		}
	}
	return len(text) - 1
}
