package hook

import (
	"bytes"
	"context"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// Conflict is what the merge git makes by itself, the merged commit and the
// commit merged into hold at a path, as bytes, nil where one holds none or
// a blob over the read bound.
type Conflict struct {
	Merged []byte
	Theirs []byte
	Ours   []byte
}

// resolution is what the bytes of a path git left unmerged are: the
// conflict git wrote or bytes git cannot read, which wait for the event of
// the resolution, a resolution the command wrote, the merged commit's side
// taken whole, or the side of HEAD kept whole, unchanged since the command
// started.
type resolution int

// The resolutions of a path git left unmerged.
const (
	waiting resolution = iota
	written
	taken
	kept
)

// markerSize is the length of the conflict markers git writes unless an
// attribute asks for longer ones.
const markerSize = 7

// resolutions sorts the paths git left unmerged after the command that git
// status did not name before it, when the command started the merge: a path
// whose bytes are those HEAD holds is kept, judged when a commit the command
// created changed it (notOwn) and left as unchanged otherwise; one whose
// bytes are those of the merged commit is named as brought by the merge;
// one whose bytes are the conflict git wrote, the merge git makes by itself
// or with one strategy option of the command (conflictKinds), waits for the
// event that writes its resolution; and one whose bytes are none of these,
// a resolution the command wrote without git add, is answered first, to be
// judged. Two texts are the same when their lines are, a carriage return
// before a line end ignored, and a conflict marker line compared by its
// character and its length alone, because git names the sides in it after
// the names a command gave them. Every path waits when the command did not
// start the merge, for an octopus merge, a path git cannot read or name,
// and when git cannot answer or no binding asks it. The bytes on disk are
// read before git is asked, so a path gone from disk or over the read bound
// waits without a merge made for it.
func (run execution) resolutions(ctx context.Context, dependencies Dependencies, before Baseline, changes Changes, unmerged []string) ([]string, []Gap) {
	theirs := strings.Fields(changes.Merge)
	if len(unmerged) == 0 || changes.Merge == before.Merge || len(theirs) != 1 || dependencies.Conflicts == nil {
		return nil, pathGaps(unmerged, causeUnmerged)
	}
	contents, read := readUnmerged(changes.Snapshot, unmerged)
	kinds := run.conflictKinds(ctx, dependencies, changes.Head, theirs[0], before.Options, contents, read)
	sorted := map[resolution][]string{}
	for _, path := range unmerged {
		sorted[kinds[path]] = append(sorted[kinds[path]], path)
	}
	stays, untold := run.notOwn(ctx, dependencies, before, changes, slices.Clone(sorted[kept]))
	own := slices.DeleteFunc(slices.Clone(sorted[kept]), among(slices.Concat(stays, untold)))
	return append(sorted[written], own...), slices.Concat(pathGaps(sorted[waiting], causeUnmerged), pathGaps(untold, causeCommits), pathGaps(sorted[taken], causeMerged))
}

// readUnmerged answers the unmerged paths whose bytes the snapshot holds
// within the read bound, with those bytes, so git makes no merge for a path
// whose bytes could not be compared.
func readUnmerged(snapshot model.Snapshot, unmerged []string) (map[string][]byte, []string) {
	contents := map[string][]byte{}
	var read []string
	for _, path := range unmerged {
		content, err := snapshot.Read(path)
		if err != nil {
			continue
		}
		contents[path], read = content, append(read, path)
	}
	return contents, read
}

// conflictKinds sorts the paths read by what their bytes are against the
// conflicts git makes of ours and theirs: the merge without an option
// first, then the merge with each strategy option of the command alone,
// asked only for the paths whose bytes no merge before it holds, because
// the text does not tell which of its merges stopped, so bytes any of them
// holds are git's, and last that merge in the base styles (restyled). A
// path git cannot answer for waits, as does every path not read.
func (run execution) conflictKinds(ctx context.Context, dependencies Dependencies, ours, theirs string, options []string, contents map[string][]byte, read []string) map[string]resolution {
	kinds := map[string]resolution{}
	candidates := make([][]string, 1, 1+len(options))
	for _, option := range options {
		candidates = append(candidates, []string{option})
	}
	left := read
	for _, candidate := range candidates {
		if len(left) == 0 {
			break
		}
		conflicts, err := dependencies.Conflicts(ctx, run.repository, ours, theirs, candidate, left)
		if err != nil {
			for _, path := range left {
				kinds[path] = waiting
			}
			break
		}
		var unmatched []string
		for _, path := range left {
			conflict, known := conflicts[path]
			if kinds[path] = resolutionOf(contents[path], conflict, known); kinds[path] == written {
				unmatched = append(unmatched, path)
			}
		}
		left = unmatched
	}
	run.restyled(ctx, dependencies, ours, theirs, contents, left, kinds)
	return kinds
}

// restyled has wait, of the paths whose bytes no merge holds, those whose
// bytes are the conflict git writes for the merge by itself in the diff3 or
// the zdiff3 style: a command that writes a conflict again picks the style
// through git checkout --conflict or a git -c of its own, which the
// configuration the merges before read does not name. Git is asked once,
// only for bytes that carry the base marker of those styles; when it cannot
// answer or no binding asks it, each such path waits too.
func (run execution) restyled(ctx context.Context, dependencies Dependencies, ours, theirs string, contents map[string][]byte, left []string, kinds map[string]resolution) {
	marked := slices.DeleteFunc(slices.Clone(left), func(path string) bool { return !baseMarked(contents[path]) })
	if len(marked) == 0 {
		return
	}
	var styled map[string][][]byte
	err := model.ErrUnreadable
	if dependencies.Restyled != nil {
		styled, err = dependencies.Restyled(ctx, run.repository, ours, theirs, marked)
	}
	for _, path := range marked {
		if err != nil || slices.ContainsFunc(styled[path], func(merged []byte) bool { return merged != nil && sameLines(contents[path], merged, true) }) {
			kinds[path] = waiting
		}
	}
}

// baseMarked reports whether bytes hold the base marker of the diff3 and
// the zdiff3 styles, a line that opens with a run of seven or more |: only
// such bytes can be a conflict in those styles, so only they cost git the
// merges of restyled.
func baseMarked(content []byte) bool {
	for line := range bytes.SplitSeq(content, []byte("\n")) {
		line = bytes.TrimSuffix(line, []byte("\r"))
		if markerRun(line) >= markerSize && line[0] == '|' {
			return true
		}
	}
	return false
}

// resolutionOf is what the bytes on disk of an unmerged path are, against
// the conflict git makes of it; bytes it could not read wait.
func resolutionOf(content []byte, conflict Conflict, read bool) resolution {
	switch {
	case !read:
		return waiting
	case conflict.Ours != nil && sameLines(content, conflict.Ours, false):
		return kept
	case conflict.Theirs != nil && sameLines(content, conflict.Theirs, false):
		return taken
	case conflict.Merged == nil || sameLines(content, conflict.Merged, true):
		return waiting
	default:
		return written
	}
}

// sameLines reports whether two texts hold the same lines, a carriage
// return before a line end ignored, and, with markers, a conflict marker
// line the same as one of the same character and length whatever follows
// it.
func sameLines(disk, git []byte, markers bool) bool {
	a, b := bytes.Split(disk, []byte("\n")), bytes.Split(git, []byte("\n"))
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := bytes.TrimSuffix(a[i], []byte("\r")), bytes.TrimSuffix(b[i], []byte("\r"))
		if !bytes.Equal(x, y) && (!markers || !sameMarker(x, y)) {
			return false
		}
	}
	return true
}

// sameMarker reports whether two lines are conflict markers of one
// character and one length, at least the length git writes by default.
func sameMarker(a, b []byte) bool {
	run := markerRun(a)
	return run >= markerSize && run == markerRun(b) && a[0] == b[0]
}

// markerRun is the length of the run of <, |, = or > a line opens with when
// the run ends the line or, for <, | and >, a space follows it, and 0 for
// any other line: git writes the name of a side or of the base after those,
// and nothing after the run of = that parts the sides, so text there is the
// command's.
func markerRun(line []byte) int {
	if len(line) == 0 || !strings.ContainsRune("<|=>", rune(line[0])) {
		return 0
	}
	n := 0
	for n < len(line) && line[n] == line[0] {
		n++
	}
	if n < len(line) && (line[0] == '=' || line[n] != ' ') {
		return 0
	}
	return n
}
