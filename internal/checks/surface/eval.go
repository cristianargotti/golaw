package surface

import (
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
	detect "github.com/cristianargotti/golaw/internal/surface"
)

const (
	evalCode = "code-eval"
	evalKey  = "S1523"
	// evalTail is the fix every message of the check states (M6.6.5).
	evalTail = " (Sonar S1523): export the code from a module and call it; writing it to a file and requiring it is the same execution."
)

// timerGlobals are the receivers through which a timer is still the global
// one.
func timerGlobals() []string { return []string{"globalThis", "window", "self"} }

// CodeEval denies a call that runs text as code: the global eval and
// Function, a timer given a string and the vm module, the Python builtins
// eval and exec, in tests too,
// because the remote analyzer reads tests as main code (A27).
func CodeEval() model.Check {
	return check{
		spec:      spec("surface.code-eval"),
		languages: []detect.Language{detect.JavaScript, detect.TypeScript, detect.JSX, detect.TSX, detect.HTML, detect.Python},
		certain:   true,
		judge:     judgeEval,
	}
}

// judgeEval routes a Python file to the builtins and every other language
// to its script units.
func judgeEval(f source, _ model.Input) []model.Finding {
	if f.info.Language == detect.Python {
		return judgePythonEval(f)
	}
	var out []model.Finding
	for _, u := range scriptUnits(f) {
		tokens := significant(u.tokens)
		vm := vmBindings(tokens)
		for i := range tokens {
			construct, first, ok := evalUse(tokens, i)
			if !ok {
				construct, first, ok = timerUse(tokens, i)
			}
			if !ok {
				construct, first, ok = vm.use(tokens, i)
			}
			if ok {
				out = append(out, evalFinding(f, tokens[first].line, construct, u.language))
			}
		}
	}
	return out
}

// evalFinding is the finding of one use, the same message in every
// language.
func evalFinding(f source, line int, construct string, language detect.Language) model.Finding {
	return f.finding(evalCode, line, "runs text as code with "+construct+evalTail, sonarKey(language, evalKey))
}

// evalUse reads the global eval or Function called at i. A member of that
// name, a function declared under it and a method definition, whose
// parameter list is followed by its body or its return type, are not the
// global.
func evalUse(tokens []token, i int) (string, int, bool) {
	name := tokens[i]
	if name.kind != tokIdent || (name.text != "eval" && name.text != "Function") {
		return "", 0, false
	}
	if member(tokens, i) || isIdentAt(tokens, i-1, "function") || !isPunctAt(tokens, i+1, "(") {
		return "", 0, false
	}
	if after := closing(tokens, i+1) + 1; isPunctAt(tokens, after, "{") || isPunctAt(tokens, after, ":") {
		return "", 0, false
	}
	if isIdentAt(tokens, i-1, keywordNew) {
		return keywordNew + " " + name.text, i - 1, true
	}
	return name.text, i, true
}

// timerUse reads setTimeout or setInterval of the global object called with
// a string or a template as its first argument.
func timerUse(tokens []token, i int) (string, int, bool) {
	name := tokens[i]
	if name.kind != tokIdent || (name.text != "setTimeout" && name.text != "setInterval") {
		return "", 0, false
	}
	first := i
	if member(tokens, i) {
		if !isPunctAt(tokens, i-1, ".") || i < 2 || tokens[i-2].kind != tokIdent ||
			!slices.Contains(timerGlobals(), tokens[i-2].text) || member(tokens, i-2) {
			return "", 0, false
		}
		first = i - 2
	}
	if !isPunctAt(tokens, i+1, "(") || i+2 >= len(tokens) {
		return "", 0, false
	}
	if arg := tokens[i+2].kind; arg != tokString && arg != tokTemplate {
		return "", 0, false
	}
	return name.text + " with a string", first, true
}

// member reports whether the token at i is reached through a dot.
func member(tokens []token, i int) bool {
	return isPunctAt(tokens, i-1, ".") || isPunctAt(tokens, i-1, "?.")
}

func isPunctAt(tokens []token, i int, text string) bool {
	return i >= 0 && i < len(tokens) && tokens[i].kind == tokPunct && tokens[i].text == text
}

func isIdentAt(tokens []token, i int, text string) bool {
	return i >= 0 && i < len(tokens) && tokens[i].kind == tokIdent && tokens[i].text == text
}
