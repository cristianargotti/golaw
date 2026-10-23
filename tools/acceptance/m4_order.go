package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/install"
)

// The m4-order category (A13): golaw and a stub second kit in one hooks
// file, the kit installed before golaw and after it.
const (
	m4OrderCategory = "m4-order"
	m4KitBefore     = "second-kit-before"
	m4KitAfter      = "second-kit-after"
	m4UpgradeDir    = "m4-upgrade"
	verbInstall     = "install"
	verbUninstall   = "uninstall"
	flagAgent       = "--agent"
	commandAgent    = "agent"
)

// m4Place is where the second kit's entry and golaw's sit in the event
// list of an agent's hooks file, -1 when absent, and the kit entry's
// bytes.
type m4Place struct {
	kit, golaw int
	raw        json.RawMessage
}

// m4OrderRows is the m4-order category.
func (s *suite) m4OrderRows(ctx context.Context, _ bool) []row {
	kit, err := s.buildProgram(ctx, filepath.Join(s.root, filepath.FromSlash(m4KitSource)), m4KitName)
	rows := make([]row, 0, 2)
	for _, name := range []string{m4KitBefore, m4KitAfter} {
		check := err
		if check == nil {
			check = s.m4SecondKit(ctx, name, kit, name == m4KitBefore)
		}
		rows = append(rows, measuredM4(m4OrderCategory, name, check))
	}
	return rows
}

// m4SecondKit installs golaw and the stub second kit in one order in a
// home of its own and holds both to A13 through golaw's install, an
// upgrade from another path and golaw's uninstall: the kit's entry keeps
// its place and its bytes, golaw's is replaced in place and then removed,
// and each kit refuses what it recognizes, whatever the other answers.
func (s *suite) m4SecondKit(ctx context.Context, name, kit string, kitFirst bool) error {
	home, env, err := s.m4Home(name)
	if err != nil {
		return err
	}
	t := m4Target{home: home, binary: s.binary, env: env}
	if t.repo, err = s.m4Repository(ctx, t, name); err != nil {
		return err
	}
	if orderErr := s.m4Order(ctx, t, kit, kitFirst); orderErr != nil {
		return orderErr
	}
	before, err := s.m4Places(t, kit)
	if err != nil {
		return err
	}
	if independentErr := s.m4Independent(ctx, t, kit, true); independentErr != nil {
		return fmt.Errorf("after the install: %w", independentErr)
	}
	if t, err = s.m4UpgradeInPlace(ctx, t, name, kit, before); err != nil {
		return err
	}
	return s.m4UninstallLeavesKit(ctx, t, kit, before)
}

// m4Order writes the kit's entries and golaw's in the order of the row.
func (s *suite) m4Order(ctx context.Context, t m4Target, kit string, kitFirst bool) error {
	steps := []func() error{
		func() error { return s.m4AgentCommand(ctx, t, verbInstall) },
		func() error { return s.m4AddKits(t.home, kit, false) },
	}
	if kitFirst {
		steps = []func() error{
			func() error { return s.m4AddKits(t.home, kit, true) },
			func() error { return s.m4AgentCommand(ctx, t, verbInstall) },
		}
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	places, err := s.m4Places(t, kit)
	for agent, place := range places {
		if err == nil && (place.kit < 0 || place.golaw < 0 || (place.kit < place.golaw) != kitFirst) {
			err = fmt.Errorf("the %s hooks file holds the kit at %d and golaw at %d: %w", agent, place.kit, place.golaw, ErrAcceptance)
		}
	}
	return err
}

// m4AddKits writes the kit's entry into both hooks files.
func (s *suite) m4AddKits(home, kit string, first bool) error {
	for _, agent := range []string{agentCodex, agentCursor} {
		if err := m4AddKit(home, agent, kit, s.m4System(), first); err != nil {
			return err
		}
	}
	return nil
}

// m4AgentCommand runs golaw agent install or uninstall for Codex and for
// Cursor with the target's binary in the target's home.
func (s *suite) m4AgentCommand(ctx context.Context, t m4Target, verb string) error {
	for _, agent := range []string{agentCodex, agentCursor} {
		got := s.execute(ctx, command{
			program: t.binary, args: []string{commandAgent, verb, flagAgent, agent}, dir: t.repo, env: t.env,
		})
		if err := success(got); err != nil {
			return fmt.Errorf("agent %s --agent %s: %w", verb, agent, err)
		}
	}
	return nil
}

// m4Places reads where the kit and golaw sit in both hooks files.
func (s *suite) m4Places(t m4Target, kit string) (map[string]m4Place, error) {
	places := make(map[string]m4Place, 2)
	for _, agent := range []string{agentCodex, agentCursor} {
		entries, err := m4ReadEntries(m4HooksPath(t.home, agent), agent)
		if err != nil {
			return nil, err
		}
		place := m4Place{kit: -1, golaw: -1}
		kitLine := install.HookCommand(kit, s.m4System(), agent)
		for i := range entries {
			entry := &entries[i]
			line := m4Command(agent, s.m4System(), entry.Handler)
			switch {
			case entry.Event != m4KitEvent(agent):
			case line == kitLine:
				place.kit, place.raw = entry.Group, entry.Raw
			case m4Names(line, t.binary):
				place.golaw = entry.Group
			}
		}
		places[agent] = place
	}
	return places, nil
}
