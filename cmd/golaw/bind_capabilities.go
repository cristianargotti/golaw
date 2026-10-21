package main

import (
	"context"
	"time"

	"github.com/cristianargotti/golaw/internal/doctor"
)

// capabilitiesDoctor names the capabilities of the policy in force. A
// policy doctor cannot load leaves no line here, as managedDoctor does:
// doctor.enrollment already names it.
func (b binding) capabilitiesDoctor(ctx context.Context, root string, now time.Time) []doctor.Line {
	loaded, err := b.loaded(ctx, root, now)
	if err != nil {
		return nil
	}
	return []doctor.Line{doctor.AssessCapabilities(loaded.Policy.Capabilities)}
}
