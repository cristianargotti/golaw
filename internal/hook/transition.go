package hook

import (
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
// command is compared by the identity its bytes have now.
func (run execution) contentChanged(ctx context.Context, dependencies Dependencies, before Baseline, after map[string]string, changes Changes) []string {
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
	blobs := changes.Blobs
	if changes.Head != before.Head {
		blobs = run.blobsAt(ctx, dependencies, before.Head, appeared)
	}
	return slices.DeleteFunc(changed, func(path string) bool {
		_, named := before.Files[path]
		return !named && sameBlob(changes, path, blobs[path])
	})
}

// blobsAt asks for the blobs a commit holds at the paths, none when the
// binding cannot answer, so such a path stays changed.
func (run execution) blobsAt(ctx context.Context, dependencies Dependencies, commit string, paths []string) map[string]string {
	if dependencies.Blobs == nil {
		return nil
	}
	blobs, err := dependencies.Blobs(ctx, run.repository, commit, paths)
	if err != nil {
		return nil
	}
	return blobs
}

// sameBlob reports whether the bytes a path holds now are the blob named;
// a path without a blob, or whose bytes the snapshot does not hold, a link
// or a file over the read bound among them, is not.
func sameBlob(changes Changes, path, blob string) bool {
	if blob == "" {
		return false
	}
	content, err := changes.Snapshot.Read(path)
	return err == nil && blobName(content, len(blob)) == blob
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
