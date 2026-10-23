package source

import (
	"strconv"
	"testing"
)

// TestPrefetchWalksTheUniverseOncePerSnapshot holds the staged scan of a
// large tree near M2: the policy family prefetches on every configuration
// read, and a walk of every entry per read made a 60,000 file repository
// twice as slow. The predicate matches nothing, so no git process runs.
func TestPrefetchWalksTheUniverseOncePerSnapshot(t *testing.T) {
	t.Parallel()
	entries := make([]treeEntry, 1000)
	for i := range entries {
		entries[i] = treeEntry{path: "assets/" + strconv.Itoa(i) + ".txt", mode: regularMode, oid: strconv.Itoa(i)}
	}
	f := newFrozen(t.Context(), t.TempDir(), nil, entries)
	calls := 0
	match := func(string) bool { calls++; return false }
	for range 3 {
		f.Prefetch(match)
	}
	if calls != len(entries) {
		t.Errorf("three prefetches asked the predicate %d times, want %d, one walk of the universe", calls, len(entries))
	}
}
