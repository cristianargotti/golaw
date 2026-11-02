package dup

import (
	"fmt"
	"go/scanner"
	"go/token"
	"slices"
	"strings"
)

// automaticSemicolon is the literal the scanner gives the semicolon it
// inserts at a line end; skipping it keeps the layout of a statement from
// hiding or creating a copy.
const automaticSemicolon = "\n"

type lexeme struct {
	key  string
	line int
}

type stream struct {
	path    string
	lexemes []lexeme
}

// tokenizeAll reads the Go files only, in path order, so two measures of
// the same files answer the same blocks.
func tokenizeAll(files map[string][]byte) ([]stream, error) {
	paths := make([]string, 0, len(files))
	for path := range files {
		if strings.HasSuffix(path, ".go") {
			paths = append(paths, path)
		}
	}
	slices.Sort(paths)
	streams := make([]stream, 0, len(paths))
	for _, path := range paths {
		s, err := tokenize(path, files[path])
		if err != nil {
			return nil, err
		}
		streams = append(streams, s)
	}
	return streams, nil
}

func tokenize(path string, src []byte) (stream, error) {
	var errs scanner.ErrorList
	file := token.NewFileSet().AddFile(path, -1, len(src))
	var s scanner.Scanner
	s.Init(file, src, errs.Add, 0)
	out := stream{path: path}
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.SEMICOLON && lit == automaticSemicolon {
			continue
		}
		out.lexemes = append(out.lexemes, lexeme{key: keyOf(tok, lit), line: file.Line(pos)})
	}
	if len(errs) > 0 {
		return stream{}, fmt.Errorf("%w: %s: %w", ErrTokenize, path, *errs[0])
	}
	return out, nil
}

// keyOf keeps identifiers, because a renamed copy is a different function
// to a reader, and reduces a literal to its kind (A50), because a copy that
// differs only in a constant is still a copy to maintain.
func keyOf(tok token.Token, lit string) string {
	if tok == token.IDENT {
		return "ident " + lit
	}
	return tok.String()
}
