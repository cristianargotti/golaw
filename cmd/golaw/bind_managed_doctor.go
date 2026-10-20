package main

import (
	"context"
	"time"

	"github.com/cristianargotti/golaw/internal/doctor"
)

// managedDoctor reports the files the managed list of the policy in force
// records whose bytes drifted from the hash init wrote (section 5.3). A
// policy doctor cannot load leaves no line here: doctor.enrollment already
// names it, and a report never fails for what it only describes.
func (b binding) managedDoctor(ctx context.Context, root string, now time.Time) []doctor.Line {
	loaded, err := b.loaded(ctx, root, now)
	if err != nil {
		return nil
	}
	recorded := make(map[string]string, len(loaded.File.Managed))
	for _, managed := range loaded.File.Managed {
		recorded[managed.Path] = managed.SHA256
	}
	return doctor.AssessManaged(doctor.CollectManaged(loaded.Root, recorded))
}
