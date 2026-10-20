package main

import (
	"context"
	"errors"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

func (b binding) initOwnership(ctx context.Context, root string) ([]byte, error) {
	repo, err := source.Discover(ctx, root)
	if err != nil || repo.Head == "" {
		data, readErr := bootstrap.ReadOwnership(root)
		return data, initOperation("read checkout ownership", readErr)
	}
	revision, err := source.NewRevision(ctx, repo, repo.Head)
	if err != nil {
		return nil, initOperation("freeze committed ownership", err)
	}
	for _, name := range []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"} {
		data, readErr := revision.Read(name)
		if errors.Is(readErr, model.ErrNotInSnapshot) {
			continue
		}
		return data, initOperation("read committed ownership", readErr)
	}
	return nil, nil
}
