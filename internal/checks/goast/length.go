package goast

import (
	"bytes"
	"fmt"
	"go/token"

	"github.com/cristianargotti/golaw/internal/model"
)

const codeFileLength = "file-length"

// FileLength is goast.file-length: a Go file has at most the file lines of
// the policy. Test files keep the limit; generated files and fixtures are
// not judged.
func FileLength() model.Check {
	return check{spec: spec("file-length"), tests: true, judge: judgeFileLength}
}

func judgeFileLength(t *target, s *state) ([]model.Finding, error) {
	limit := s.in.Policy.Sizes.FileLines
	lines := countLines(t.src)
	if lines <= limit {
		return nil, nil
	}
	message := fmt.Sprintf("file has %d lines, the limit is %d: split it by responsibility", lines, limit)
	return []model.Finding{t.finding(codeFileLength, token.NoPos, message)}, nil
}

// countLines counts the lines of a file the way a file set does: one per
// newline, plus the last line when it does not end in a newline.
func countLines(src []byte) int {
	if len(src) == 0 {
		return 0
	}
	n := bytes.Count(src, []byte{'\n'})
	if src[len(src)-1] != '\n' {
		n++
	}
	return n
}
