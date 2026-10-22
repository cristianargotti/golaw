package policyfiles

import (
	"context"
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
)

// buildCheck is a check of the build half: its specification and a pure
// judge over the snapshot, so the checks of the half share one Run that
// honors cancellation and never runs a process.
type buildCheck struct {
	metadata model.Spec
	judge    func(model.Input) model.Result
}

// Spec answers the specification the engine registers.
func (c buildCheck) Spec() model.Spec { return c.metadata }

// Run judges the input unless the run was canceled first, because a
// canceled scan must never answer a verdict it did not finish.
func (c buildCheck) Run(ctx context.Context, in model.Input) (model.Result, error) {
	if err := ctx.Err(); err != nil {
		return model.Result{}, fmt.Errorf("%s: %w", c.metadata.ID, err)
	}
	return c.judge(in), nil
}

// buildSnapshot retains the failing read path without interpreting error text.
// An included file can exceed the bound even when the root is small.
type buildSnapshot struct {
	model.Snapshot

	failed string
}

// Read reads through the snapshot and remembers the path of a failed read,
// which the result names as unread when the read bound refused it.
func (s *buildSnapshot) Read(path string) ([]byte, error) {
	data, err := s.Snapshot.Read(path)
	if err != nil {
		s.failed = path
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

// Select forwards the listing of the wrapped snapshot, so an include glob
// keeps the snapshot's own selection instead of a full listing.
func (s *buildSnapshot) Select(match func(string) bool) []model.File {
	return model.Select(s.Snapshot, match)
}

// Prefetch forwards the batched load of the wrapped snapshot, so a read
// through this wrapper still asks git once for the configuration.
func (s *buildSnapshot) Prefetch(match func(string) bool) {
	model.Prefetch(s.Snapshot, match)
}

// Any forwards the existence test of the wrapped snapshot for the same reason.
func (s *buildSnapshot) Any(match func(string) bool) bool {
	return model.Any(s.Snapshot, match)
}
