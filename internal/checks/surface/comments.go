package surface

import (
	"bytes"
	"go/scanner"
	gotoken "go/token"
	"slices"
	"strings"

	detect "github.com/cristianargotti/golaw/internal/surface"
)

// The marks of a block comment of scripts and styles.
const (
	blockOpen  = "/*"
	blockClose = "*/"
)

// comment is one comment of a file: its text with its marks, the 1-based
// line it starts on, and the language it is written in, empty for Go.
type comment struct {
	text     string
	line     int
	language detect.Language
}

// comments answers every comment of a file in the languages the family
// lexes, in the order of their lines.
func comments(f source) []comment {
	if strings.HasSuffix(f.path, goSuffix) {
		return goComments(f.src)
	}
	switch f.info.Language {
	case detect.JavaScript, detect.TypeScript, detect.JSX, detect.TSX:
		return scriptComments(f)
	case detect.HTML:
		return htmlComments(f)
	case detect.CSS:
		return styleComments(f.src, 1)
	case detect.Python:
		return pythonComments(f.src)
	case detect.Shell, detect.Dockerfile, detect.YAML, detect.CloudFormation:
		return hashComments(f.src, f.info.Language)
	}
	return nil
}

// goComments reads the comments the Go scanner returns. Its errors are
// ignored, so a file that does not parse still yields its comments.
func goComments(src []byte) []comment {
	fset := gotoken.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	var s scanner.Scanner
	s.Init(file, src, nil, scanner.ScanComments)
	var out []comment
	for {
		pos, tok, lit := s.Scan()
		if tok == gotoken.EOF {
			return out
		}
		if tok == gotoken.COMMENT {
			out = append(out, comment{text: lit, line: fset.Position(pos).Line})
		}
	}
}

// scriptComments reads the comment tokens of every script unit.
func scriptComments(f source) []comment {
	var out []comment
	for _, u := range scriptUnits(f) {
		for _, t := range u.tokens {
			if t.kind == tokComment {
				out = append(out, comment{text: t.text, line: t.line, language: u.language})
			}
		}
	}
	return out
}

// htmlComments joins the comments of the scripts, of the markup outside
// every region and of the style regions, ordered by line.
func htmlComments(f source) []comment {
	out := scriptComments(f)
	out = append(out, markupComments(f.src, f.info.Regions)...)
	for _, r := range f.info.Regions {
		if r.Kind == detect.RegionStyle {
			out = append(out, styleComments(f.src[r.Start:max(r.Start, r.End)], r.StartLine)...)
		}
	}
	slices.SortStableFunc(out, func(a, b comment) int { return a.line - b.line })
	return out
}

// markupComments reads the HTML comments outside the bodies of the
// regions; an unterminated comment runs to the end.
func markupComments(src []byte, regions []detect.Region) []comment {
	var out []comment
	pos, next := 0, 0
	for pos < len(src) {
		if next < len(regions) && pos >= regions[next].Start {
			pos = max(pos, regions[next].End)
			next++
			continue
		}
		if !bytes.HasPrefix(src[pos:], []byte(markupCommentOpen)) {
			pos++
			continue
		}
		end := bytes.Index(src[pos:], []byte(markupCommentClose))
		stop := len(src)
		if end >= 0 {
			stop = pos + end + len(markupCommentClose)
		}
		out = append(out, comment{text: string(src[pos:stop]), line: 1 + bytes.Count(src[:pos], []byte("\n")), language: detect.HTML})
		pos = stop
	}
	return out
}

// styleComments reads the block comments of a style sheet whose first line
// is firstLine. Smallest behavior: a string holding the opening mark is
// not read as a string, so it and its rest are read as a comment, a stated
// limit the suppression marker is never part of.
func styleComments(src []byte, firstLine int) []comment {
	var out []comment
	pos := 0
	for {
		start := bytes.Index(src[pos:], []byte(blockOpen))
		if start < 0 {
			return out
		}
		start += pos
		stop := len(src)
		if end := bytes.Index(src[start+len(blockOpen):], []byte(blockClose)); end >= 0 {
			stop = start + len(blockOpen) + end + len(blockClose)
		}
		line := firstLine + bytes.Count(src[:start], []byte("\n"))
		out = append(out, comment{text: string(src[start:stop]), line: line, language: detect.CSS})
		pos = stop
	}
}

// pythonComments reads the comment tokens of the Python lexer.
func pythonComments(src []byte) []comment {
	var out []comment
	for _, t := range lexPython(src) {
		if t.kind == tokComment {
			out = append(out, comment{text: t.text, line: t.line, language: detect.Python})
		}
	}
	return out
}

// hashComments reads the whole lines whose first byte that is not a blank
// is a hash, but a shebang on the first line. Smallest behavior: a hash
// after code may sit inside a quoted word, and a certain denial never reads
// a guess, so only whole-line comments are read.
func hashComments(src []byte, language detect.Language) []comment {
	var out []comment
	for i, raw := range bytes.Split(src, []byte("\n")) {
		text := strings.TrimSpace(strings.TrimSuffix(string(raw), "\r"))
		if !strings.HasPrefix(text, "#") || (i == 0 && strings.HasPrefix(text, "#!")) {
			continue
		}
		out = append(out, comment{text: text, line: i + 1, language: language})
	}
	return out
}
