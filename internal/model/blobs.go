package model

import "fmt"

// Blobs is a snapshot that also reads a blob by its object id, so a check
// compares the two sides of a staged change through the snapshot alone.
type Blobs interface {
	Blob(id string) ([]byte, error)
}

// ReadBlob reads a blob through a snapshot that holds blobs; any other
// snapshot answers ErrUnreadable, so the check reports what it could not
// read instead of judging half a change.
func ReadBlob(s Snapshot, id string) ([]byte, error) {
	blobs, ok := s.(Blobs)
	if !ok {
		return nil, fmt.Errorf("%w: blob %s: the snapshot holds no blobs", ErrUnreadable, id)
	}
	data, err := blobs.Blob(id)
	if err != nil {
		return nil, fmt.Errorf("read blob %s: %w", id, err)
	}
	return data, nil
}
