package profile

import (
	"slices"
	"testing"
	"time"
)

// recordClock is the fixed clock of the record tests of this package.
func recordClock() time.Time { return time.Date(2026, time.October, 21, 12, 0, 0, 0, time.UTC) }

func versionsOf(r Record) []string {
	out := make([]string, 0, len(r.Versions))
	for _, i := range r.Versions {
		out = append(out, i.Version)
	}
	return out
}

func TestWithCurrentKeepsTheCurrentVersionLast(t *testing.T) {
	t.Parallel()
	first := Installed{Name: "fixture", Version: "1.2.3", Digest: "sha256:aa"}
	second := Installed{Name: "fixture", Version: "1.3.0", Digest: "sha256:bb"}
	r := Record{}.withCurrent(first, recordClock())
	r = r.withCurrent(second, recordClock().Add(time.Hour))
	if got := versionsOf(r); !slices.Equal(got, []string{"1.2.3", "1.3.0"}) || r.Current != "1.3.0" || r.fault("fixture") != "" {
		t.Fatalf("two installs = %v, current %s, fault %q", got, r.Current, r.fault("fixture"))
	}
	again := r.withCurrent(first, recordClock().Add(2*time.Hour))
	if got := versionsOf(again); !slices.Equal(got, []string{"1.3.0", "1.2.3"}) || again.Current != "1.2.3" || !again.Updated.Equal(recordClock().Add(2*time.Hour)) {
		t.Fatalf("a version made current again moves last: %v, %s, %v", got, again.Current, again.Updated)
	}
	if got := versionsOf(r); !slices.Equal(got, []string{"1.2.3", "1.3.0"}) {
		t.Fatalf("withCurrent changed the record it was called on: %v", got)
	}
}

func TestEntryAndPreviousReadTheOrder(t *testing.T) {
	t.Parallel()
	r := Record{}.withCurrent(Installed{Name: "fixture", Version: "1.2.3"}, recordClock())
	if _, ok := r.previous(); ok {
		t.Fatal("one version has no previous one")
	}
	r = r.withCurrent(Installed{Name: "fixture", Version: "1.3.0"}, recordClock())
	if prev, ok := r.previous(); !ok || prev.Version != "1.2.3" {
		t.Fatalf("previous = %+v, %t", prev, ok)
	}
	if got, ok := r.entry("1.3.0"); !ok || got.Version != "1.3.0" {
		t.Fatalf("entry = %+v, %t", got, ok)
	}
	if _, ok := r.entry("9.9.9"); ok {
		t.Fatal("an absent version has no entry")
	}
}
