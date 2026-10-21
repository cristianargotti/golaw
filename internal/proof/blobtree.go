package proof

import (
	"bytes"
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
)

// BlobTree keeps base blobs separate so they cannot enter the candidate tree.
type BlobTree struct {
	*Tree

	blobs map[string][]byte
}

// NewBlobTree copies both sources so later fixture edits cannot change a run.
func NewBlobTree(files, blobs map[string][]byte) *BlobTree {
	t := &BlobTree{Tree: NewTree(files), blobs: make(map[string][]byte, len(blobs))}
	for id, content := range blobs {
		t.blobs[id] = bytes.Clone(content)
	}
	return t
}

// Blob copies the bounded bytes so one check cannot change another's evidence.
func (t *BlobTree) Blob(id string) ([]byte, error) {
	content, ok := t.blobs[id]
	if !ok {
		return nil, fmt.Errorf("%w: blob %s", model.ErrUnreadable, id)
	}
	if len(content) > model.MaxReadBytes {
		return nil, fmt.Errorf("%w: blob %s", model.ErrTooLarge, id)
	}
	return bytes.Clone(content), nil
}
