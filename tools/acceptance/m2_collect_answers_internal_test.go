package main

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
)

// m2CollectionScript answers every row of the collection by the case its
// command runs in, with the answer of that row's family in one mode; a
// command outside a case goes to the fixture executor.
func m2CollectionScript(t *testing.T, s *suite, mode string) *m2Script {
	t.Helper()
	answers := make(map[string]m2Answer)
	return newM2Script(t, s, func(ctx context.Context, c command) (m2Reply, bool) {
		name := m2CaseName(s, c)
		answer, found := answers[name]
		if !found {
			answer = m2RowAnswer(t, s, name, mode)
			answers[name] = answer
		}
		if answer == nil {
			return m2Reply{}, false
		}
		return answer(ctx, c)
	})
}

// m2RowAnswer is the answer of the catalog a row belongs to, or nil for a
// name no catalog holds.
func m2RowAnswer(t *testing.T, s *suite, name, mode string) m2Answer {
	t.Helper()
	switch {
	case slices.Contains(m2StagedNames(), name):
		return m2StagedAnswer(t, s, name, mode)
	case slices.Contains(m2LifecycleNames(), name):
		return m2LifecycleAnswer(t, s, name, mode)
	case slices.Contains(m2NativeNames(), name):
		return m2NativeAnswer(t, s, name, mode)
	}
	return nil
}

// m2CaseName is the case a command runs in: m2Scope gives the case named
// n the home <shared base>/cases/n/home, and a command outside a case has
// none.
func m2CaseName(s *suite, c command) string {
	home := environmentValue(c.env, "HOME")
	if filepath.Dir(filepath.Dir(home)) != filepath.Join(s.m2.Base, "cases") {
		return ""
	}
	return filepath.Base(filepath.Dir(home))
}
