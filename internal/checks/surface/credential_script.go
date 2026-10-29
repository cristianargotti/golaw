package surface

import (
	"github.com/cristianargotti/golaw/internal/model"
	detect "github.com/cristianargotti/golaw/internal/surface"
)

// The forms a literal takes as the value of a name.
const (
	formAssigns = "assigns"
	formPasses  = "passes"
	formBinds   = "binds"
	// entryOffset is how far before a value the opener of its entry sits:
	// the opener, the key, the colon, the value.
	entryOffset   = 3
	keywordFrom   = "from"
	keywordImport = "import"
)

// opensEntry reports the brace or the comma before the key of an entry.
func opensEntry(tokens []token, j int) bool {
	return isPunctAt(tokens, j, "{") || isPunctAt(tokens, j, ",")
}

// judgeJSCredential reads the literals of one script unit. A literal a
// credential name receives is one finding; any other literal, unless it is
// a key or an import source, is read for a pair or a URL inside it, so one
// literal gives at most one finding.
func judgeJSCredential(f source, u unit) []model.Finding {
	tokens := significant(u.tokens)
	jsx := u.language == detect.JSX || u.language == detect.TSX
	var out []model.Finding
	for i, t := range tokens {
		if t.kind != tokString && (t.kind != tokTemplate || t.sub) {
			continue
		}
		value := stringValue(t)
		if form, name, ok := nameValue(tokens, i, jsx); ok && credentialName(name) && !benign(name, value) {
			out = append(out, nameFinding(f, form, name, t.line, u.language))
			continue
		}
		if objectKey(tokens, i) || isIdentAt(tokens, i-1, keywordFrom) || isIdentAt(tokens, i-1, keywordImport) {
			continue
		}
		out = append(out, literalFindings(f, value, t.line, u.language)...)
	}
	return out
}

// nameValue answers the name the literal at i is the value of and the
// form: an assignment (after a type annotation of one identifier too), a
// JSX attribute, or an object entry. The literal must end its expression,
// so a concatenation or a call on it is a value built at run time.
func nameValue(tokens []token, i int, jsx bool) (string, string, bool) {
	if isPunctAt(tokens, i-1, "=") {
		n := i - 2
		if n >= 2 && tokens[n].kind == tokIdent && isPunctAt(tokens, n-1, ":") && tokens[n-2].kind == tokIdent {
			n -= 2
		}
		if n < 0 || tokens[n].kind != tokIdent {
			return "", "", false
		}
		if jsx && insideElement(tokens, n) {
			return formPasses, tokens[n].text, true
		}
		return formAssigns, tokens[n].text, ends(tokens, i)
	}
	if !isPunctAt(tokens, i-1, ":") || !opensEntry(tokens, i-entryOffset) ||
		!ends(tokens, i) {
		return "", "", false
	}
	if key := tokens[i-2]; key.kind == tokIdent {
		return formBinds, key.text, true
	} else if key.kind == tokString {
		return formBinds, stringValue(key), true
	}
	return "", "", false
}

// ends reports whether the literal at i ends its expression: the unit
// ends, the next token is on a later line, or it closes a statement, an
// entry or a bracket.
func ends(tokens []token, i int) bool {
	if i+1 >= len(tokens) || tokens[i+1].line > tokens[i].line {
		return true
	}
	next := tokens[i+1]
	if next.kind != tokPunct {
		return false
	}
	switch next.text {
	case ";", ",", ")", "]", "}":
		return true
	}
	return false
}

// insideElement reports whether the name at n is an attribute of a JSX
// element: walking back over attribute names, values and expression
// containers reaches the element's tag before anything else.
func insideElement(tokens []token, n int) bool {
	for j := n - 1; j >= 0; j-- {
		t := tokens[j]
		switch {
		case t.kind == tokTag:
			return true
		case t.kind == tokIdent, t.kind == tokString, isPunctAt(tokens, j, "="):
		case isPunctAt(tokens, j, "}"):
			j = opening(tokens, j)
		default:
			return false
		}
	}
	return false
}

// opening answers the index of the brace that opens the container closed
// at end, 0 when none does.
func opening(tokens []token, end int) int {
	depth := 0
	for j := end; j >= 0; j-- {
		switch {
		case isPunctAt(tokens, j, "}"):
			depth++
		case isPunctAt(tokens, j, "{"):
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return 0
}

// objectKey reports a string that is the key of an object entry.
func objectKey(tokens []token, i int) bool {
	return isPunctAt(tokens, i+1, ":") && opensEntry(tokens, i-1)
}
