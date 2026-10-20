package rung

import (
	"context"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
)

// placedLine reads the file and the line go names at the start of a line it
// writes for a problem, path:line:column: message; the shortest path wins,
// so a Windows drive letter stays part of the path.
var placedLine = regexp.MustCompile(`^(.+?):(\d+):`)

// introduced answers, by path, the lines the commit adds: a rung refuses
// only what the commit introduces, and a line the staged patch adds is what
// golangci-lint keeps through --new-from-patch. While a merge is in progress
// that patch also holds what the merge brought, so a line counts only when
// the index adds it against the other parent too, as the staged scan
// compares with both.
func (p *precommit) introduced(ctx context.Context) (map[string]map[int]bool, error) {
	added, err := p.addedLines(ctx)
	theirs := p.scan.Index.MergeHead()
	if err != nil || theirs == "" {
		return added, err
	}
	other, err := p.addedLines(ctx, theirs)
	if err != nil {
		return nil, err
	}
	for file, lines := range added {
		maps.DeleteFunc(lines, func(line int, _ bool) bool { return !other[file][line] })
	}
	return added, nil
}

// addedLines answers, by path, the lines the index adds against HEAD or the
// commit named, read without context, so every line a hunk adds is one it
// marks.
func (p *precommit) addedLines(ctx context.Context, against ...string) (map[string]map[int]bool, error) {
	diff, err := p.stagedDiff(ctx, slices.Concat([]string{"-U0"}, against)...)
	if err != nil {
		return nil, err
	}
	added := map[string]map[int]bool{}
	file, hunks := "", false
	for line := range strings.SplitSeq(string(diff), "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			file, hunks = "", false
		case !hunks && strings.HasPrefix(line, "+++ b/"):
			// git ends the name with a tab when it holds a space.
			file = strings.TrimSuffix(strings.TrimPrefix(line, "+++ b/"), "\t")
		case strings.HasPrefix(line, "@@ "):
			hunks = true
			mark(added, file, line)
		}
	}
	return added, nil
}

// mark records the lines one hunk adds to a file, from the new side of its
// header, @@ -a,b +c,d @@, where git leaves out a count of one; a file git
// names no new side for, a deletion or a pure rename, adds none.
func mark(added map[string]map[int]bool, file, header string) {
	fields := strings.Fields(header)
	if file == "" || len(fields) < 3 {
		return
	}
	first, count, counted := strings.Cut(strings.TrimPrefix(fields[2], "+"), ",")
	start, n := number(first), 1
	if counted {
		n = number(count)
	}
	for line := start; line < start+n; line++ {
		if added[file] == nil {
			added[file] = map[int]bool{}
		}
		added[file][line] = true
	}
}

// number reads a count git writes in decimal; what git never writes reads
// as zero, which marks no line.
func number(text string) int {
	n, err := strconv.Atoi(text)
	if err != nil {
		return 0
	}
	return n
}

// attributed splits what a tool printed about the problems it places into
// those on a line the staged patch adds and the rest: a line indented by a
// tab continues the problem above it, the package header go prints over a
// build error is dropped, and a line no position starts is never the
// commit's, because nothing places it in what the commit adds.
func attributed(out runner.Output, added map[string]map[int]bool) ([]string, []string) {
	var kept, inherited []string
	ours := false
	for _, line := range toolLines(out) {
		switch {
		case strings.HasPrefix(line, "# "):
			continue
		case !strings.HasPrefix(line, "\t"):
			ours = false
			if m := placedLine.FindStringSubmatch(line); m != nil {
				ours = added[strings.ReplaceAll(m[1], `\`, "/")][number(m[2])]
			}
		}
		if ours {
			kept = append(kept, line)
		} else {
			inherited = append(inherited, line)
		}
	}
	return kept, inherited
}
