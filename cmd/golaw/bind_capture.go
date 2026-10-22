package main

import (
	"context"
	"fmt"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/install"
)

// capture records the key names of a payload the adapter could not judge
// yet (A4) under the state directory, keyed by the project identity, for
// doctor to report; the hook core drops its error, because the record is
// evidence, never a verdict.
func (h hookBinding) capture(_ context.Context, repo hook.Repository, c hook.Capture) error {
	_, identity, err := install.ProjectIdentity(repo.Root)
	if err != nil {
		return fmt.Errorf("the project identity of %s: %w", repo.Root, err)
	}
	record := doctor.CaptureRecord{Identity: identity, HookDigest: h.hookDigest, Captures: []hook.Capture{c}, SeenAt: h.now}
	if err := doctor.SaveCapture(agents.CapturePath(h.state, h.agent, identity), record); err != nil {
		return fmt.Errorf("record the capture: %w", err)
	}
	return nil
}
