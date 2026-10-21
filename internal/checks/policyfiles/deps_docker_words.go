package policyfiles

import "strings"

const dockerSpace = " \t\r\n"

func dockerLogicalLine(lines []string, index *int) (string, bool) {
	text := lines[*index]
	for dockerContinued(text) {
		*index++
		if *index >= len(lines) {
			return "", false
		}
		next := lines[*index]
		if trimmed := strings.TrimSpace(next); trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		text = strings.TrimSuffix(strings.TrimRight(text, " \t"), "\\") + next
	}
	return text, true
}

// BuildKit continues on a trailing escape whatever precedes it.
func dockerContinued(text string) bool {
	return strings.HasSuffix(strings.TrimRight(text, " \t"), "\\")
}

func dockerToken(text string) (string, string, bool) {
	var word strings.Builder
	var quote byte
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case quote == 0 && strings.ContainsRune(dockerSpace, rune(c)):
			return word.String(), text[i:], true
		case c == '\\' && quote != '\'':
			if quote == '"' && i+1 < len(text) && !strings.ContainsRune("$\"\\", rune(text[i+1])) {
				word.WriteByte(c)
				continue
			}
			i++
			if i >= len(text) {
				return "", "", false
			}
			word.WriteByte(text[i])
		case c == quote:
			quote = 0
		case quote == 0 && (c == '\'' || c == '"'):
			quote = c
		default:
			word.WriteByte(c)
		}
	}
	return word.String(), "", quote == 0
}

func dockerWords(text string) ([]string, bool) {
	var words []string
	for text = strings.TrimSpace(text); text != ""; text = strings.TrimSpace(text) {
		word, rest, ok := dockerToken(text)
		if !ok {
			return nil, false
		}
		words = append(words, word)
		text = rest
	}
	return words, true
}

func dockerExpand(text string, args map[string]string) (string, bool) {
	var expanded strings.Builder
	for {
		prefix, rest, found := strings.Cut(text, "$")
		expanded.WriteString(prefix)
		if !found {
			return expanded.String(), true
		}
		name, suffix, valid := dockerVariable(rest)
		value, known := args[name]
		if !valid || !known || strings.Contains(value, "$") {
			return "", false
		}
		expanded.WriteString(value)
		text = suffix
	}
}

func dockerArguments(text string, args map[string]string) {
	for text = strings.TrimSpace(text); text != ""; text = strings.TrimSpace(text) {
		word, rest, ok := dockerToken(text)
		if !ok {
			return
		}
		dockerArgument(word, text[:len(text)-len(rest)], args)
		text = rest
	}
}

func dockerArgument(word, raw string, args map[string]string) {
	name, value, hasDefault := strings.Cut(word, "=")
	if !dockerIdentifier(name) || !hasDefault {
		return
	}
	if strings.Contains(raw, "$") && strings.ContainsAny(raw, "'\\") {
		value = raw
	} else if expanded, resolved := dockerExpand(value, args); resolved {
		value = expanded
	}
	args[name] = value
}

func dockerVariable(text string) (string, string, bool) {
	if strings.HasPrefix(text, "{") {
		name, rest, closed := strings.Cut(text[1:], "}")
		return name, rest, closed && dockerIdentifier(name)
	}
	end := 0
	for end < len(text) && dockerNameByte(text[end]) {
		end++
	}
	return text[:end], text[end:], dockerIdentifier(text[:end])
}

func dockerIdentifier(name string) bool {
	if name == "" || (name[0] >= '0' && name[0] <= '9') {
		return false
	}
	for i := range len(name) {
		if !dockerNameByte(name[i]) {
			return false
		}
	}
	return true
}

func dockerNameByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}

type dockerHeredoc struct {
	word string
	tabs bool
}

// A line that does not lex holds no heredoc unless it shows <<, because
// BuildKit ignores the lexing error when it looks for one.
func dockerHeredocs(text string) ([]dockerHeredoc, bool) {
	if !strings.Contains(text, "<<") {
		return nil, true
	}
	var bodies []dockerHeredoc
	for text = strings.TrimSpace(text); text != ""; text = strings.TrimSpace(text) {
		_, rest, ok := dockerToken(text)
		if !ok {
			return nil, false
		}
		body, valid := dockerHereWord(text[:len(text)-len(rest)])
		if !valid {
			return nil, false
		}
		if body != nil {
			bodies = append(bodies, *body)
		}
		text = rest
	}
	return bodies, true
}

func dockerHereWord(raw string) (*dockerHeredoc, bool) {
	word := strings.TrimLeft(raw, "0123456789")
	word, found := strings.CutPrefix(word, "<<")
	if !found || strings.Contains(word, "<") {
		return nil, true
	}
	word, tabs := strings.CutPrefix(word, "-")
	if word == "" {
		return nil, true
	}
	name, rest, ok := dockerToken(word)
	if !ok || rest != "" || dockerComputedDelimiter(word) {
		return nil, false
	}
	return &dockerHeredoc{word: name, tabs: tabs}, true
}

func dockerComputedDelimiter(word string) bool {
	var quote byte
	for index := 0; index < len(word); index++ {
		switch c := word[index]; {
		case c == '\\' && quote != '\'':
			index++
		case c == quote:
			quote = 0
		case quote == 0 && (c == '\'' || c == '"'):
			quote = c
		case quote != '\'' && (c == '$' || c == '`'):
			return true
		}
	}
	return false
}

func dockerBodies(lines []string, index *int, text string) bool {
	command, rest := dockerCommand(text)
	if command == "ONBUILD" {
		command, rest = dockerCommand(rest)
	}
	if command != "RUN" && command != "COPY" && command != "ADD" {
		return true
	}
	bodies, ok := dockerHeredocs(rest)
	if !ok {
		return false
	}
	for _, body := range bodies {
		for *index++; *index < len(lines); *index++ {
			line := lines[*index]
			if body.tabs {
				line = strings.TrimLeft(line, "\t")
			}
			if line == body.word {
				break
			}
		}
		if *index == len(lines) {
			return false
		}
	}
	return true
}
