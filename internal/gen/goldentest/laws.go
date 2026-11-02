package goldentest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Law is one rule of the Sonar green record as a golden assertion: a
// template whose rendering breaks it would fail the remote gate on its
// first run, so the kit refuses it before any repository sees it.
type Law struct {
	// Name is the id a proof manifest binds its fixtures to.
	Name string
	// Assert answers one message per violation in the rendered files,
	// keyed by slash path; none means the files keep the law.
	Assert func(files map[string][]byte) []string
}

// The names of the laws, in the order of the rule table of M7.10.2.
const (
	lawS107        = "S107-parameters"
	lawS1192       = "S1192-literals"
	lawRegexp      = "no-regexp"
	lawHTTP        = "no-http-literal"
	lawAddresses   = "doc-ranges-only"
	lawSuppression = "no-suppression"
	lawModes       = "file-modes"
	lawS2068       = "no-credential-literal"
	lawContext     = "context-from-test"
)

// The thresholds Sonar's way profile applies to Go.
const (
	maxParameters     = 7
	literalRepeats    = 3
	literalMinLength  = 5
	interpretedQuotes = '"'
)

// Laws answers every template law, in the order of the rule table.
func Laws() []Law {
	return []Law{
		lawOf(lawS107, judgeParameters),
		lawOf(lawS1192, judgeLiterals),
		lawOf(lawRegexp, judgeRegexp),
		lawOf(lawHTTP, judgeHTTP),
		lawOf(lawAddresses, judgeAddresses),
		lawOf(lawSuppression, judgeSuppression),
		lawOf(lawModes, judgeModes),
		lawOf(lawS2068, judgeCredentials),
		lawOf(lawContext, judgeContext),
	}
}

// lawOf binds a judge of one parsed file to the files a kind rendered; a
// file that does not parse is a violation of every law, because no law
// can be shown kept on it.
func lawOf(name string, judge func(f goFile) []string) Law {
	return Law{Name: name, Assert: func(files map[string][]byte) []string {
		parsed, out := goFiles(files)
		for _, f := range parsed {
			out = append(out, judge(f)...)
		}
		return out
	}}
}

// goFile is one parsed Go file of a rendering.
type goFile struct {
	path string
	fset *token.FileSet
	file *ast.File
	test bool
}

// goFiles parses the Go files in path order, so the messages of a law
// come out in the same order on every run.
func goFiles(files map[string][]byte) ([]goFile, []string) {
	paths := make([]string, 0, len(files))
	for path := range files {
		if strings.HasSuffix(path, ".go") {
			paths = append(paths, path)
		}
	}
	slices.Sort(paths)
	var parsed []goFile
	var failures []string
	for _, path := range paths {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, files[path], parser.ParseComments)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: does not parse: %v", path, err))
			continue
		}
		parsed = append(parsed, goFile{path: path, fset: fset, file: file, test: strings.HasSuffix(path, "_test.go")})
	}
	return parsed, failures
}

// at formats one violation at its line.
func at(f goFile, pos token.Pos, law, detail string) string {
	return fmt.Sprintf("%s:%d %s: %s", f.path, f.fset.Position(pos).Line, law, detail)
}

// judgeParameters is go:S107: a function type of every form, declaration,
// literal or interface method, takes at most seven parameters.
func judgeParameters(f goFile) []string {
	var out []string
	ast.Inspect(f.file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncType)
		if !ok || fn.Params == nil {
			return true
		}
		count := 0
		for _, field := range fn.Params.List {
			count += max(1, len(field.Names))
		}
		if count > maxParameters {
			detail := fmt.Sprintf("%d parameters, at most %d: take the port values or one Deps or Config struct", count, maxParameters)
			out = append(out, at(f, fn.Pos(), lawS107, detail))
		}
		return true
	})
	return out
}

// literalCount keeps the first position, where the violation sits.
type literalCount struct {
	first token.Pos
	count int
}

// judgeLiterals is go:S1192 the way goast.sonar-literal-repeat reads it,
// in non-test files only because Sonar runs the rule with scope MAIN.
func judgeLiterals(f goFile) []string {
	if f.test {
		return nil
	}
	counts := map[string]*literalCount{}
	tags := map[*ast.BasicLit]bool{}
	ast.Inspect(f.file, func(n ast.Node) bool {
		if field, ok := n.(*ast.Field); ok && field.Tag != nil {
			tags[field.Tag] = true
		}
		lit, ok := n.(*ast.BasicLit)
		if !ok || tags[lit] || lit.Kind != token.STRING || lit.Value[0] != interpretedQuotes {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil || utf8.RuneCountInString(value) <= literalMinLength || bareWord(value) {
			return true
		}
		if counts[value] == nil {
			counts[value] = &literalCount{first: lit.Pos()}
		}
		counts[value].count++
		return true
	})
	return repeated(f, counts)
}

// repeated answers the literals at the threshold in source order; the
// message never carries the value, which can be a credential.
func repeated(f goFile, counts map[string]*literalCount) []string {
	var hits []literalCount
	for _, c := range counts {
		if c.count >= literalRepeats {
			hits = append(hits, *c)
		}
	}
	slices.SortFunc(hits, func(a, b literalCount) int { return int(a.first - b.first) })
	out := make([]string, 0, len(hits))
	for _, c := range hits {
		detail := fmt.Sprintf("a string literal appears %d times: name it once in the regenerable constants", c.count)
		out = append(out, at(f, c.first, lawS1192, detail))
	}
	return out
}

// bareWord reports a value of ASCII letters, digits and underscores only,
// the \w class goast.sonar-literal-repeat reads, which Sonar never counts
// as a duplicated literal.
func bareWord(value string) bool {
	for _, r := range value {
		if r != '_' && (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}
