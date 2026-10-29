package surface

import (
	"fmt"
	"path"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	detect "github.com/cristianargotti/golaw/internal/surface"
)

// PythonFunctionLength reports a Python function body over the function
// lines of the policy, read by indentation and counted as funlen counts
// with comments ignored. It is feedback, because the body boundary is read
// by indentation and not by a parser.
func PythonFunctionLength() model.Check {
	return check{
		spec:      spec("surface.python-function-length"),
		languages: []detect.Language{detect.Python},
		judge:     judgeFunctionLength,
	}
}

// keywordDef opens a function definition.
const keywordDef = "def"

// definition is one def: its name, its line, the column its line starts
// at and the index of the colon that ends its header.
type definition struct {
	name   string
	line   int
	indent int
	header int
}

// judgeFunctionLength reports every definition whose body is over the
// limit; a pytest file is a test file, which loses the function limit
// (spec section 3).
func judgeFunctionLength(f source, in model.Input) []model.Finding {
	if pytestFile(f.path) {
		return nil
	}
	tokens := lexPython(f.src)
	starts := lineStarts(tokens)
	limit := in.Policy.Sizes.FunctionLines
	var out []model.Finding
	for _, d := range definitions(tokens, starts) {
		if n := counted(tokens, d, bodyEnd(tokens, starts, d)); n > limit {
			message := fmt.Sprintf("function %s is %d lines, the limit is %d: give it one job.", d.name, n, limit)
			out = append(out, f.finding("python-function-length", d.line, message, ""))
		}
	}
	return out
}

// pytestFile reports the names pytest collects as tests, and its conftest.
func pytestFile(name string) bool {
	base := path.Base(name)
	return strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py") || base == "conftest.py"
}

// lineStarts marks the tokens that start a logical line: the first token
// of a line outside brackets that is not a comment, so a comment never
// ends a body and a continuation inside brackets never does either.
func lineStarts(tokens []pyToken) []bool {
	starts := make([]bool, len(tokens))
	depth := 0
	for i, t := range tokens {
		first := i == 0 || tokens[i-1].endLine < t.line
		starts[i] = first && depth == 0 && t.kind != tokComment
		if t.kind == tokPunct {
			depth = max(0, depth+bracketStep(t.text))
		}
	}
	return starts
}

// definitions answers the defs that start their line, after an async that
// does too.
func definitions(tokens []pyToken, starts []bool) []definition {
	var out []definition
	for i, t := range tokens {
		if t.kind != tokIdent || t.text != keywordDef || i+1 >= len(tokens) {
			continue
		}
		first := i
		if i > 0 && tokens[i-1].text == "async" && tokens[i-1].line == t.line {
			first = i - 1
		}
		if !starts[first] {
			continue
		}
		if h := headerEnd(tokens, i); h >= 0 {
			out = append(out, definition{name: tokens[i+1].text, line: t.line, indent: tokens[first].col, header: h})
		}
	}
	return out
}

// headerEnd answers the index of the first colon at bracket depth zero
// after the def at i, -1 when the header never ends.
func headerEnd(tokens []pyToken, i int) int {
	depth := 0
	for j := i + 1; j < len(tokens); j++ {
		t := tokens[j]
		if t.kind != tokPunct {
			continue
		}
		if t.text == ":" && depth == 0 {
			return j
		}
		depth += bracketStep(t.text)
	}
	return -1
}

// bodyEnd answers the index one past the last token of the body: the
// first line start after the header line at or under the indentation.
func bodyEnd(tokens []pyToken, starts []bool, d definition) int {
	headerLine := tokens[d.header].endLine
	for j := d.header + 1; j < len(tokens); j++ {
		if starts[j] && tokens[j].line > headerLine && tokens[j].col <= d.indent {
			return j
		}
	}
	return len(tokens)
}

// counted answers the lines from the one after the header to the last
// line a code token of the body covers, minus the lines that hold only
// comments; blank lines inside count, as funlen counts them.
func counted(tokens []pyToken, d definition, end int) int {
	first, last := tokens[d.header].endLine+1, 0
	code := map[int]bool{}
	for _, t := range tokens[d.header+1 : end] {
		if t.kind == tokComment {
			continue
		}
		for l := t.line; l <= t.endLine; l++ {
			code[l] = true
		}
		last = max(last, t.endLine)
	}
	if last < first {
		return 0
	}
	comments := map[int]bool{}
	for _, t := range tokens[d.header+1 : end] {
		if t.kind == tokComment && !code[t.line] && t.line >= first && t.line <= last {
			comments[t.line] = true
		}
	}
	return last - first + 1 - len(comments)
}
