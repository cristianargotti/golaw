package hook

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // git names a blob by the sha1 of its bytes; the name is compared, never trusted
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
)

// contentChanged is what changed between the identities of the pre event
// and of the post event, less the paths only their status moved: a path
// git status named after the command and not before it was clean then, so
// its bytes were the blob the HEAD of the pre event held there, and it is
// unchanged when its bytes are still that blob. A path named before the
// command is compared by the identity its bytes have now, and a path the
// move of HEAD changed that git status reports clean after it is changed.
func (run execution) contentChanged(ctx context.Context, dependencies Dependencies, before Baseline, after map[string]string, changes Changes, moved moves) []string {
	changed := Changed(before.Files, after)
	var appeared []string
	for _, path := range changed {
		if _, named := before.Files[path]; !named {
			appeared = append(appeared, path)
		}
	}
	if len(appeared) == 0 || before.Head == "" {
		return changed
	}
	blobs, settled := changes.Blobs, map[string]bool(nil)
	if changes.Head != before.Head {
		blobs, settled = moved.blobs(changes)
	}
	same := run.sameBlobs(ctx, dependencies, changes, appeared, blobs, settled)
	return slices.DeleteFunc(changed, func(path string) bool { return same[path] })
}

// sameBlobs are the paths whose bytes are still the blob named for them:
// by the name of the bytes on disk, or, when git may convert them on their
// way in, by the name git gives them through that conversion, asked once
// for every such path. A path git status found edited under its own mode
// was compared by git through its conversions, so unless its bytes hold a
// carriage return, which an end of line rule may drop on a change of size
// alone, it changed and git is not asked, and so did a path settled, one
// git already found apart from its blob. A path without a blob, or whose
// bytes the snapshot does not hold, a link or a file over the read bound
// among them, is not the blob.
func (run execution) sameBlobs(ctx context.Context, dependencies Dependencies, changes Changes, paths []string, blobs map[string]string, settled map[string]bool) map[string]bool {
	same := map[string]bool{}
	var converted []string
	for _, path := range paths {
		content, err := changes.Snapshot.Read(path)
		switch blob := blobs[path]; {
		case blob == "" || err != nil:
		case blobName(content, len(blob)) == blob:
			same[path] = true
		case settled[path]:
		case !changes.Edited[path] || bytes.IndexByte(content, '\r') >= 0:
			converted = append(converted, path)
		}
	}
	if len(converted) == 0 || dependencies.Hashes == nil {
		return same
	}
	hashes, err := dependencies.Hashes(ctx, run.repository, converted)
	if err != nil {
		return same
	}
	for _, path := range converted {
		same[path] = hashes[path] == blobs[path]
	}
	return same
}

// blobName is the name git gives a blob of these bytes in the object
// format a name of that length belongs to, empty for any other length.
func blobName(content []byte, length int) string {
	header := "blob " + strconv.Itoa(len(content)) + "\x00"
	object := append(append(make([]byte, 0, len(header)+len(content)), header...), content...)
	switch length {
	case sha1.Size * 2:
		sum := sha1.Sum(object) //nolint:gosec // git names a blob by the sha1 of its bytes; the name is compared, never trusted
		return hex.EncodeToString(sum[:])
	case sha256.Size * 2:
		sum := sha256.Sum256(object)
		return hex.EncodeToString(sum[:])
	default:
		return ""
	}
}
