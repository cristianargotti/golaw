package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/install"
)

// The owners of the two handlers an order row sends commands through.
const (
	ownerGolaw = "golaw"
	ownerKit   = "kit"
)

// m4Independent sends golaw's red command and the kit's red command
// through each owner's handler: golaw refuses its own and lets the kit's
// through, the kit refuses its own and lets golaw's through, so each
// refusal stands alone and deny wins in either order. Without golaw the
// kit is checked alone.
func (s *suite) m4Independent(ctx context.Context, t m4Target, kit string, withGolaw bool) error {
	for _, agent := range []string{agentCodex, agentCursor} {
		golawRed, err := s.m4ShellPayload(agent, t.repo, m4SkipHooks)
		if err != nil {
			return err
		}
		kitRed, err := s.m4ShellPayload(agent, t.repo, "echo "+m4KitMarker)
		if err != nil {
			return err
		}
		lines := map[string]string{ownerKit: install.HookCommand(kit, s.m4System(), agent)}
		if withGolaw {
			event, tool := m4RouteOf(agent, golawRed)
			entry, golawErr := s.m4Golaw(t, agent, event, tool)
			if golawErr != nil {
				return golawErr
			}
			lines[ownerGolaw] = m4Command(agent, s.m4System(), entry.Handler)
		}
		for owner, line := range lines {
			if err := s.m4RefusesOwn(ctx, t, agent, owner, line, map[string][]byte{ownerGolaw: golawRed, ownerKit: kitRed}); err != nil {
				return err
			}
		}
	}
	return nil
}

// m4RefusesOwn requires the handler of an owner to block the red command
// of that owner and to let the other owner's through, on every shell.
func (s *suite) m4RefusesOwn(ctx context.Context, t m4Target, agent, owner, line string, reds map[string][]byte) error {
	for redOwner, payload := range reds {
		replies, err := s.m4Through(ctx, t, agent, line, payload)
		if err != nil {
			return err
		}
		for _, reply := range replies {
			blocked, hostErr := m4HostBlocks(agent, reply.got)
			if hostErr != nil {
				return fmt.Errorf("the %s handler of %s through %s: %w", owner, agent, reply.shell, hostErr)
			}
			if blocked != (redOwner == owner) {
				return fmt.Errorf("the %s handler of %s answered blocked %v to the command of %s: %w", owner, agent, blocked, redOwner, ErrAcceptance)
			}
		}
	}
	return nil
}

// m4UpgradeInPlace installs golaw again from a copy of its binary at
// another path, a new version installed elsewhere: golaw's entries are
// replaced where they were, the kit's keep their place and bytes, and both
// still refuse independently.
func (s *suite) m4UpgradeInPlace(ctx context.Context, t m4Target, name, kit string, before map[string]m4Place) (m4Target, error) {
	body, err := readFile(t.binary)
	if err != nil {
		return t, err
	}
	t.binary = filepath.Join(s.base, m4UpgradeDir, name, executable(binaryName))
	if writeErr := writeFile(t.binary, body, execMode); writeErr != nil {
		return t, writeErr
	}
	if installErr := s.m4AgentCommand(ctx, t, verbInstall); installErr != nil {
		return t, installErr
	}
	after, err := s.m4Places(t, kit)
	if err != nil {
		return t, err
	}
	for agent, place := range after {
		was := before[agent]
		if place.golaw != was.golaw || place.kit != was.kit || !sameJSON(place.raw, was.raw) {
			return t, fmt.Errorf("the upgrade moved an entry of %s: the kit from %d to %d, golaw from %d to %d: %w",
				agent, was.kit, place.kit, was.golaw, place.golaw, ErrAcceptance)
		}
	}
	if independentErr := s.m4Independent(ctx, t, kit, true); independentErr != nil {
		return t, fmt.Errorf("after the upgrade: %w", independentErr)
	}
	return t, nil
}

// m4UninstallLeavesKit removes golaw with the upgraded binary: golaw's
// entries are gone, the kit's keep their bytes, and the kit alone still
// refuses its command and allows golaw's.
func (s *suite) m4UninstallLeavesKit(ctx context.Context, t m4Target, kit string, before map[string]m4Place) error {
	if err := s.m4AgentCommand(ctx, t, verbUninstall); err != nil {
		return err
	}
	after, err := s.m4Places(t, kit)
	if err != nil {
		return err
	}
	for agent, place := range after {
		if place.golaw >= 0 || place.kit < 0 || !sameJSON(place.raw, before[agent].raw) {
			return fmt.Errorf("the uninstall left golaw at %d or changed the kit entry of %s: %w", place.golaw, agent, ErrAcceptance)
		}
	}
	if err := s.m4Independent(ctx, t, kit, false); err != nil {
		return fmt.Errorf("after the uninstall: %w", err)
	}
	return nil
}
