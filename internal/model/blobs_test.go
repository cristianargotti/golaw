package model_test

import (
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

// blobbed is a snapshot that holds blobs by object id, as the index does.
type blobbed struct {
	plain

	blobs map[string][]byte
}

func (b blobbed) Blob(id string) ([]byte, error) {
	data, ok := b.blobs[id]
	if !ok {
		return nil, model.ErrNotInSnapshot
	}
	return data, nil
}

func TestReadBlobAsksASnapshotThatHoldsBlobs(t *testing.T) {
	t.Parallel()
	s := blobbed{blobs: map[string][]byte{"e69de29": []byte("old\n")}}
	got, err := model.ReadBlob(s, "e69de29")
	if err != nil || string(got) != "old\n" {
		t.Errorf("ReadBlob = %q, %v", got, err)
	}
	if _, err := model.ReadBlob(s, "absent"); !errors.Is(err, model.ErrNotInSnapshot) {
		t.Errorf("absent blob: %v, want ErrNotInSnapshot", err)
	}
	if _, err := model.ReadBlob(plain{}, "e69de29"); !errors.Is(err, model.ErrUnreadable) {
		t.Errorf("snapshot without blobs: %v, want ErrUnreadable", err)
	}
}
