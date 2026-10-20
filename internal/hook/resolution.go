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
// created changed it (notOwn) and left as unchanged otherwise; one whose bytes are
// those of the merged commit is named as brought by the merge; one whose
// bytes are the conflict git wrote, the merge git makes by itself with the
// strategy options of the command, waits for the event that writes its
// resolution; and one whose bytes are none of these, a resolution the
// command wrote without git add, is answered first, to be judged. Two texts
// are the same when their lines are, a carriage return before a line end
// ignored, and a conflict marker line compared by its character and its
// length alone, because git names the sides in it after the names a command
// gave them. Every path waits when the command did not start the merge, for
// an octopus merge, a path git cannot read or name, and when git cannot
// answer or no binding asks it. The bytes on disk are read before git is
// asked, so a path gone from disk or over the read bound waits without a
// merge made for it.
func (run execution) resolutions(ctx context.Context, dependencies Dependencies, before Baseline, changes Changes, unmerged []string) ([]string, []Gap) {
	theirs := strings.Fields(changes.Merge)
	if len(unmerged) == 0 || changes.Merge == before.Merge || len(theirs) != 1 || dependencies.Conflicts == nil {
		return nil, pathGaps(unmerged, causeUnmerged)
	}
	contents, read := readUnmerged(changes.Snapshot, unmerged)
	var conflicts map[string]Conflict
	if len(read) > 0 {
		var err error
		if conflicts, err = dependencies.Conflicts(ctx, run.repository, changes.Head, theirs[0], before.Options, read); err != nil {
			return nil, pathGaps(unmerged, causeUnmerged)
		}
	}
	sorted := map[resolution][]string{}
	for _, path := range unmerged {
		content, held := contents[path]
		conflict, known := conflicts[path]
		kind := resolutionOf(content, conflict, held && known)
		sorted[kind] = append(sorted[kind], path)
	}
	stays, untold := run.notOwn(ctx, dependencies, before, changes, slices.Clone(sorted[kept]))
	own := slices.DeleteFunc(slices.Clone(sorted[kept]), among(slices.Concat(stays, untold)))
	return append(sorted[written], own...), slices.Concat(pathGaps(sorted[waiting], causeUnmerged), pathGaps(untold, causeCommits), pathGaps(sorted[taken], causeMerged))
}

// readUnmerged reads the bytes on disk of each unmerged path the snapshot
// holds within the read bound and answers the paths it read, in order.
func readUnmerged(snapshot model.Snapshot, unmerged []string) (map[string][]byte, []string) {
	contents := map[string][]byte{}
	var read []string
	for _, path := range unmerged {
		if content, err := snapshot.Read(path); err == nil {
			contents[path], read = content, append(read, path)
		}
	}
	return contents, read
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
// the run ends the line or a space follows it, and 0 for any other line.
func markerRun(line []byte) int {
	if len(line) == 0 || !strings.ContainsRune("<|=>", rune(line[0])) {
		return 0
	}
	n := 0
	for n < len(line) && line[n] == line[0] {
		n++
	}
	if n < len(line) && line[n] != ' ' {
		return 0
	}
	return n
}
