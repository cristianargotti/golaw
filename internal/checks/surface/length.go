package surface

import (
	"bytes"
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
	detect "github.com/cristianargotti/golaw/internal/surface"
)

// FileLength denies a hand-written script, Python or shell file over the
// file lines of the policy when the candidate crosses the limit: the Go
// size law holds for the other languages of the repository (M6.7 check 3).
// Markup, styles and data keep no length law.
func FileLength() model.Check {
	return check{
		spec: spec("surface.file-length"),
		languages: []detect.Language{
			detect.JavaScript, detect.TypeScript, detect.JSX, detect.TSX, detect.Python, detect.Shell,
		},
		certain: true,
		byCode:  true,
		judge:   judgeFileLength,
	}
}

// judgeFileLength answers one finding for a file over the limit, at line
// 0 because the file as a whole is over it.
func judgeFileLength(f source, in model.Input) []model.Finding {
	limit := in.Policy.Sizes.FileLines
	n := countLines(f.src)
	if n <= limit {
		return nil
	}
	message := fmt.Sprintf("has %d lines, the limit is %d: split it by responsibility.", n, limit)
	return []model.Finding{f.finding("file-length", 0, message, "")}
}

// countLines counts as goast.file-length counts: one per line break, plus
// a last line without one. A carriage return is no line break, so a
// Windows checkout counts the same.
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
