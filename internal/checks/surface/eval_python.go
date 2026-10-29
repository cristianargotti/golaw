package surface

import (
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
	detect "github.com/cristianargotti/golaw/internal/surface"
)

// pyBuiltins are the Python builtins that run text as code.
func pyBuiltins() []string { return []string{"eval", "exec"} }

// judgePythonEval reads the calls of the builtins eval and exec. A file
// that binds either name itself (a def, a class, an assignment at the top
// of a statement or an import) is judged as not calling that builtin
// anywhere: a stated limit that only misses, because a method named eval
// cannot be told from a call of the builtin of the same file without
// scopes.
func judgePythonEval(f source) []model.Finding {
	tokens := pySignificant(lexPython(f.src))
	shadow := shadowed(tokens)
	var out []model.Finding
	for i, t := range tokens {
		if t.kind != tokIdent || !slices.Contains(pyBuiltins(), t.text) || shadow[t.text] {
			continue
		}
		if i > 0 && slices.Contains([]string{".", "def", "class"}, tokens[i-1].text) {
			continue
		}
		if i+1 < len(tokens) && tokens[i+1].kind == tokPunct && tokens[i+1].text == "(" {
			out = append(out, evalFinding(f, t.line, t.text, detect.Python))
		}
	}
	return out
}

// shadowed answers the builtin names the file binds itself.
func shadowed(tokens []pyToken) map[string]bool {
	shadow := map[string]bool{}
	depth := 0
	for i, t := range tokens {
		if t.kind == tokPunct {
			depth += bracketStep(t.text)
		}
		switch {
		case t.kind != tokIdent:
		case (t.text == "def" || t.text == "class") && i+1 < len(tokens):
			shadow[tokens[i+1].text] = true
		case t.text == "import":
			for _, name := range importedNames(tokens, i) {
				shadow[name] = true
			}
		case depth == 0 && i+1 < len(tokens) && tokens[i+1].kind == tokPunct && tokens[i+1].text == "=":
			shadow[t.text] = true
		}
	}
	return shadow
}

// bracketStep is the change of bracket depth an operator makes.
func bracketStep(text string) int {
	switch text {
	case "(", "[", "{":
		return 1
	case ")", "]", "}":
		return -1
	}
	return 0
}

// importedNames answers the names the import at i binds: each entry's name
// after as, else its first name. The entries run to the end of the line,
// or to the closing parenthesis of a parenthesized list.
func importedNames(tokens []pyToken, i int) []string {
	var names []string
	entry := []pyToken{}
	flush := func() {
		if len(entry) >= 3 && entry[len(entry)-2].text == "as" {
			names = append(names, entry[len(entry)-1].text)
		} else if len(entry) > 0 {
			names = append(names, entry[0].text)
		}
		entry = entry[:0]
	}
	paren := i+1 < len(tokens) && tokens[i+1].text == "("
	for j := i + 1; j < len(tokens); j++ {
		t := tokens[j]
		if (!paren && t.line != tokens[j-1].endLine) || t.text == ")" || t.text == ";" {
			break
		}
		switch {
		case t.text == ",":
			flush()
		case t.kind == tokIdent:
			entry = append(entry, t)
		}
	}
	flush()
	return names
}
