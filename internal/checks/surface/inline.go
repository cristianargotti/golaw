package surface

import (
	"bytes"
	"fmt"
	"path"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	detect "github.com/cristianargotti/golaw/internal/surface"
)

// inlineLimit is the most lines an inline script holds (M6.7 check 4).
const inlineLimit = 40

// InlineScriptSize denies an inline script of an HTML file over 40 lines
// when the candidate crosses the limit: a script moved into a file the Go
// side embeds is read whole by every linter.
func InlineScriptSize() model.Check {
	return check{
		spec:      spec("surface.inline-script-size"),
		languages: []detect.Language{detect.HTML},
		certain:   true,
		byCode:    true,
		judge:     judgeInline,
	}
}

// judgeInline answers one finding per script region over the limit, at
// the line of its opening tag, spliced regions included.
func judgeInline(f source, _ model.Input) []model.Finding {
	var out []model.Finding
	for _, r := range f.info.Regions {
		if r.Kind != detect.RegionScript {
			continue
		}
		n := codeLines(f.src[r.Start:max(r.Start, r.End)])
		if n <= inlineLimit {
			continue
		}
		moved := strings.TrimSuffix(f.path, path.Ext(f.path)) + ".js"
		message := fmt.Sprintf("holds an inline script of %d lines, the limit is %d: move it to %s, embed it with go:embed and splice it when the view is served, so the linters read it.", n, inlineLimit, moved)
		out = append(out, f.finding("inline-script-size", tagLine(f.src, r.Start), message, ""))
	}
	return out
}

// codeLines counts the lines from the first to the last line of a body
// that holds a byte other than a blank. Smallest behavior: the blank lines
// around a body are layout, so 40 statements between the tags count 40
// whatever line breaks surround them; blank lines inside count.
func codeLines(body []byte) int {
	first, last := 0, 0
	for i, line := range bytes.Split(body, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if first == 0 {
			first = i + 1
		}
		last = i + 1
	}
	if first == 0 {
		return 0
	}
	return last - first + 1
}

// tagLine answers the line of the opening tag before a body: the last
// less-than sign before it opens the tag, because the body starts right
// after that tag closes.
func tagLine(src []byte, start int) int {
	open := max(0, bytes.LastIndexByte(src[:start], '<'))
	return 1 + bytes.Count(src[:open], []byte("\n"))
}
