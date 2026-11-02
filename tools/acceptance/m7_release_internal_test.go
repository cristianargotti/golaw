package main

import (
	"slices"
	"testing"
)

func TestM7RowsJoinTheReleaseRunAfterTheM6Rows(t *testing.T) {
	t.Parallel()
	calls := m6Calls(t)
	m6, m7 := slices.Index(calls, "m6AcceptanceRows"), slices.Index(calls, "m7AcceptanceRows")
	if m6 < 0 || m7 < m6 {
		t.Fatalf("runAcceptance calls %q: the M7 rows must follow the M6 rows", calls)
	}
	if slices.Index(calls[m7+1:], "m7AcceptanceRows") >= 0 {
		t.Fatalf("runAcceptance calls %q: the M7 rows run once", calls)
	}
}
