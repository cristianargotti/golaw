package main

import (
	"context"
	"testing"
)

// TestM2RowsNameTheCandidateTheyVerified holds every passing M2 row to its
// own detail: setup-clean installs the pinned tools from the module proxy
// and the candidate build may reach it, so the M1 detail "verified offline"
// would misstate the evidence of an M2 row, while M1 keeps it.
func TestM2RowsNameTheCandidateTheyVerified(t *testing.T) {
	t.Parallel()
	const want = "verified against the candidate"
	for _, tt := range []struct {
		category, name, mode, status string
	}{
		{m2StagedCategory, "partial-near", "answer", statusPass},
		{m2LifecycleCategory, "init-unarmed", "answer", statusPass},
		{m2RungCategory, "installed-commit-msg-near", "answer", statusPass},
		{m2RungCategory, "installed-commit-msg-near", "broken", statusFail},
	} {
		s := m2TestSuite(t)
		s.execute = m2CollectionScript(t, s, tt.mode).Execute
		item := s.m2CollectedRow(t.Context(), tt.category, tt.name)
		if item.status != tt.status || !item.required || (item.detail == want) != (tt.status == statusPass) || item.detail == "" {
			t.Fatalf("%s/%s %s: row=%+v", tt.category, tt.name, tt.mode, item)
		}
	}
	s := m2TestSuite(t)
	s.execute = func(context.Context, command) result { return result{stdout: m2CompleteMarker + "\n"} }
	if item := s.measurementM2(t.Context()); item.status != statusPass || item.detail != want {
		t.Fatalf("measurement row=%+v", item)
	}
	if item := measured("protocol", "allow", nil); item.detail != "verified offline" {
		t.Fatalf("M1 row=%+v", item)
	}
}
