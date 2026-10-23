package main

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// m4OrderState is what the rows of one home ran before a call: the agent
// installs and whether golaw was uninstalled.
type m4OrderState struct {
	installs    int
	uninstalled bool
}

// m4OrderFault answers one call of an order row with a defect instead of
// the reply m4OrderSuite gives it.
type m4OrderFault func(c command, state m4OrderState) (result, bool)

// m4FaultyOrder is an order suite whose calls fault picks first, given
// the state of the home of the call.
func m4FaultyOrder(t *testing.T, fault m4OrderFault) *suite {
	t.Helper()
	s := m4OrderSuite(t, m4Installer{}, false)
	script, states := s.execute, map[string]*m4OrderState{}
	s.execute = func(ctx context.Context, c command) result {
		home := environmentValue(c.env, "HOME")
		if states[home] == nil {
			states[home] = &m4OrderState{}
		}
		if got, faulted := fault(c, *states[home]); faulted {
			return got
		}
		if len(c.args) == 4 && c.args[0] == commandAgent {
			states[home].installs++
			states[home].uninstalled = states[home].uninstalled || c.args[1] == verbUninstall
		}
		return script(ctx, c)
	}
	return s
}

func TestM4OrderRowsNameTheStepThatFailed(t *testing.T) {
	t.Parallel()
	agentCall := func(c command, verb string) bool {
		return len(c.args) == 4 && c.args[0] == commandAgent && c.args[1] == verb
	}
	golawShell := func(c command) bool {
		return len(c.args) == 2 && c.args[0] == "-c" && !strings.Contains(c.args[1], m4KitName)
	}
	for _, tc := range []struct {
		name, want string
		fault      m4OrderFault
	}{
		{"red an upgrade that fails", "agent install --agent codex", func(c command, state m4OrderState) (result, bool) {
			return result{code: 1}, agentCall(c, verbInstall) && state.installs >= 2
		}},
		{"red an uninstall that fails", "agent uninstall --agent codex", func(c command, _ m4OrderState) (result, bool) {
			return result{code: 1}, agentCall(c, verbUninstall)
		}},
		{"red a golaw hook that fails", "a failed hook", func(c command, _ m4OrderState) (result, bool) {
			return result{code: 1}, golawShell(c)
		}},
		{"red a kit that stops refusing after the uninstall", "after the uninstall", func(c command, state m4OrderState) (result, bool) {
			line := c.args[len(c.args)-1]
			return result{}, state.uninstalled && strings.Contains(line, m4KitName) && !strings.HasSuffix(line, agentCursor)
		}},
	} {
		s := m4FaultyOrder(t, tc.fault)
		rows := s.m4OrderRows(t.Context(), false)
		if !slices.ContainsFunc(rows, func(item row) bool { return item.status == statusFail && strings.Contains(item.detail, tc.want) }) {
			t.Errorf("%s: %+v", tc.name, rows)
		}
	}
	s := m4OrderSuite(t, m4Installer{}, false)
	s.lookup = func(name string) (string, error) {
		if name == "sh" {
			return "", ErrAcceptance
		}
		return name, nil
	}
	for _, item := range s.m4OrderRows(t.Context(), false) {
		if item.status != statusFail || !strings.Contains(item.detail, "locate sh") {
			t.Errorf("a missing shell: %+v", item)
		}
	}
}
