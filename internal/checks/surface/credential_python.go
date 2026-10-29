package surface

import (
	"github.com/cristianargotti/golaw/internal/model"
	detect "github.com/cristianargotti/golaw/internal/surface"
)

// judgePythonCredential reads the literals of a Python file as the script
// reader does; an f-string with a substitution is built at run time and is
// never a literal.
func judgePythonCredential(f source) []model.Finding {
	tokens := pySignificant(lexPython(f.src))
	opens := enclosing(tokens)
	var out []model.Finding
	for i, t := range tokens {
		if t.kind != tokString {
			continue
		}
		value, literal := pyStringValue(t)
		if !literal {
			continue
		}
		if form, name, ok := pyNameValue(tokens, i, opens[i]); ok && credentialName(name) && !benign(name, value) {
			out = append(out, nameFinding(f, form, name, t.line, detect.Python))
			continue
		}
		if isPy(tokens, i+1, ":") && pyOpensEntry(tokens, i-1) {
			continue
		}
		out = append(out, literalFindings(f, value, t.line, detect.Python)...)
	}
	return out
}

// pyNameValue answers the form and the name the literal at i is the value
// of: an assignment at the top of a statement (after an annotation of
// dotted names too), a keyword argument inside parentheses, or a dict
// entry with a string key; open is the innermost bracket around it.
func pyNameValue(tokens []pyToken, i int, open string) (string, string, bool) {
	if !pyEnds(tokens, i) {
		return "", "", false
	}
	if isPy(tokens, i-1, "=") {
		n := annotated(tokens, i-2)
		if n < 0 || tokens[n].kind != tokIdent {
			return "", "", false
		}
		switch open {
		case "":
			return formAssigns, tokens[n].text, true
		case "(":
			return formPasses, tokens[n].text, true
		}
		return "", "", false
	}
	if isPy(tokens, i-1, ":") && i >= entryOffset && tokens[i-2].kind == tokString &&
		pyOpensEntry(tokens, i-entryOffset) {
		if key, literal := pyStringValue(tokens[i-2]); literal {
			return formBinds, key, true
		}
	}
	return "", "", false
}

// annotated answers the index of the name an annotation at n follows, as
// in name: pkg.Type = value, or n itself when no annotation is there.
func annotated(tokens []pyToken, n int) int {
	j := n
	for j >= 2 && tokens[j].kind == tokIdent && isPy(tokens, j-1, ".") && tokens[j-2].kind == tokIdent {
		j -= 2
	}
	if j >= 2 && tokens[j].kind == tokIdent && isPy(tokens, j-1, ":") && tokens[j-2].kind == tokIdent {
		return j - 2
	}
	return n
}

// pyEnds reports whether the literal at i ends its expression; an if or an
// else after it makes it one branch of a conditional, which is no end.
func pyEnds(tokens []pyToken, i int) bool {
	if i+1 >= len(tokens) || tokens[i+1].line > tokens[i].endLine {
		return true
	}
	next := tokens[i+1]
	if next.kind != tokPunct {
		return false
	}
	switch next.text {
	case ",", ")", "]", "}":
		return true
	}
	return false
}

// enclosing answers, for each token, the innermost bracket open around it,
// empty at depth zero.
func enclosing(tokens []pyToken) []string {
	out := make([]string, len(tokens))
	var stack []string
	for i, t := range tokens {
		if len(stack) > 0 {
			out[i] = stack[len(stack)-1]
		}
		if t.kind != tokPunct {
			continue
		}
		switch t.text {
		case "(", "[", "{":
			stack = append(stack, t.text)
		case ")", "]", "}":
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	return out
}

func isPy(tokens []pyToken, i int, text string) bool {
	return i >= 0 && i < len(tokens) && tokens[i].kind == tokPunct && tokens[i].text == text
}

// pyOpensEntry reports the brace or the comma before the key of an entry.
func pyOpensEntry(tokens []pyToken, j int) bool { return isPy(tokens, j, "{") || isPy(tokens, j, ",") }
