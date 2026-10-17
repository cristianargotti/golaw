package hook

import (
	"context"
	"maps"
)

// moves is what the move of HEAD since the pre event changed: each path
// that differs between the tree the HEAD of the pre event held and the tree
// HEAD holds after the command, the empty tree on an unborn branch, with the
// blob the earlier one held there, empty where it held none. known is false
// when HEAD moved and git could not answer the move, or no binding asks it.
type moves struct {
	paths map[string]string
	known bool
}

// movedSince asks once, and only when HEAD moved, what the move since the
// HEAD of the pre event changed, an unborn branch at either end included.
func (run execution) movedSince(ctx context.Context, dependencies Dependencies, since string) moves {
	now := run.repository.Head
	if since == now {
		return moves{known: true}
	}
	if dependencies.Moved == nil {
		return moves{}
	}
	paths, err := dependencies.Moved(ctx, run.repository, since, now)
	if err != nil {
		return moves{}
	}
	return moves{paths: paths, known: true}
}

// blobs answers the blob the HEAD of the pre event held at the paths git
// status named after the move, those of the commits apart from the blobs
// HEAD holds now, and the paths the move changed that git status reports
// clean after it, whose bytes git already found apart from that blob.
func (m moves) blobs(changes Changes) (map[string]string, map[string]bool) {
	blobs := maps.Clone(changes.Blobs)
	if blobs == nil {
		blobs = map[string]string{}
	}
	named := among(changes.Paths)
	settled := map[string]bool{}
	for path, blob := range m.paths {
		blobs[path] = blob
		settled[path] = !named(path)
	}
	return blobs, settled
}
