package install

import (
	"encoding/base64"
	"encoding/binary"
	"slices"
	"strings"
	"unicode/utf16"
)

// decodeLauncher reads the executable and the arguments out of an encoded
// launcher script, refusing a malformed encoding or an altered script;
// the caller regenerates the command from them before trusting the pair.
func decodeLauncher(encoded string) (string, []string, bool) {
	script, ok := decodeScript(encoded)
	if !ok {
		return "", nil, false
	}
	executable, arguments, ok := scriptLiterals(script)
	if !ok {
		return "", nil, false
	}
	args, ok := crtWords(arguments)
	if !ok {
		return "", nil, false
	}
	return executable, args, true
}

// decodeScript answers the text of a base64 UTF-16LE script. An odd byte
// count, or a surrogate that encodes back differently from how it was
// read, is no script PowerShell would run as written, so it is refused.
func decodeScript(encoded string) (string, bool) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) == 0 || len(data)%2 != 0 {
		return "", false
	}
	units := make([]uint16, len(data)/2)
	for index := range units {
		units[index] = binary.LittleEndian.Uint16(data[2*index:])
	}
	text := string(utf16.Decode(units))
	if !slices.Equal(utf16.Encode([]rune(text)), units) {
		return "", false
	}
	return text, true
}

// scriptLiterals reads the two string literals of the launcher script,
// the executable and the arguments, out of the fixed text around them.
func scriptLiterals(script string) (string, string, bool) {
	rest, found := strings.CutPrefix(script, scriptOpen)
	if !found {
		return "", "", false
	}
	executable, rest, found := cutLiteral(rest, scriptArgs)
	if !found {
		return "", "", false
	}
	arguments, rest, found := cutLiteral(rest, scriptClose)
	if !found || rest != "" {
		return "", "", false
	}
	return executable, arguments, true
}

// cutLiteral reads a single-quoted literal that the separator follows and
// answers its text with the apostrophes undoubled.
func cutLiteral(text, separator string) (string, string, bool) {
	literal, rest, found := strings.Cut(text, separator)
	if !found || len(literal) < 2 || literal[0] != '\'' || literal[len(literal)-1] != '\'' {
		return "", "", false
	}
	return strings.ReplaceAll(literal[1:len(literal)-1], "''", "'"), rest, true
}

// crtWords splits a command line the way the C runtime of a child does,
// the inverse of crtJoin on what it writes: whitespace separates the words
// outside quotes, a quote toggles them, and a run of backslashes reads as
// crtBackslashes says. An open quote at the end is refused.
func crtWords(line string) ([]string, bool) {
	var words []string
	var word []byte
	inWord, quoted := false, false
	index := 0
	for index < len(line) {
		c := line[index]
		switch {
		case c == '\\':
			var text []byte
			text, quoted, index = crtBackslashes(line, index, quoted)
			word = append(word, text...)
			inWord = true
		case c == '"':
			quoted, inWord = !quoted, true
			index++
		case (c == ' ' || c == '\t') && !quoted:
			if inWord {
				words = append(words, string(word))
				word, inWord = word[:0], false
			}
			index++
		default:
			word = append(word, c)
			inWord = true
			index++
		}
	}
	if quoted {
		return nil, false
	}
	if inWord {
		words = append(words, string(word))
	}
	return words, true
}

// crtBackslashes reads a run of backslashes at index: before a quote, 2n
// of them are n and the quote toggles the quoted state, 2n+1 are n and a
// literal quote; elsewhere they are kept as they are. It answers the text,
// the quoted state and the index after what it read.
func crtBackslashes(line string, index int, quoted bool) ([]byte, bool, int) {
	end := index
	for end < len(line) && line[end] == '\\' {
		end++
	}
	count := end - index
	if end == len(line) || line[end] != '"' {
		return []byte(line[index:end]), quoted, end
	}
	text := []byte(strings.Repeat(`\`, count/2))
	if count%2 == 1 {
		return append(text, '"'), quoted, end + 1
	}
	return text, !quoted, end + 1
}
