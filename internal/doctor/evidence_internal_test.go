package doctor

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// lineOf is the one line of an assessment with an id.
func lineOf(t *testing.T, input Snapshot, id string) Line {
	t.Helper()
	var found []Line
	for _, line := range Assess(input).Lines {
		if line.ID == id {
			found = append(found, line)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s lines = %#v", id, found)
	}
	return found[0]
}

// TestVersionAgainstTheValidatedHost compares the host with the floor and
// with the validated version (section 9): below the floor is red, up to
// the validated version supported, and a newer host is unknown, because
// nothing proved the adapter against it yet.
func TestVersionAgainstTheValidatedHost(t *testing.T) {
	t.Parallel()
	for version, want := range map[string]string{
		"2.1.294 (Claude Code)": stateRed, "2.1.295 (Claude Code)": stateOK,
		"2.1.299 (Claude Code)": stateOK, "2.1.301 (Claude Code)": stateUnknown,
	} {
		input := baseline(t)
		input.ClaudeVersion = version
		line := lineOf(t, input, "doctor.version")
		if line.State != want || !strings.Contains(line.Detail, "validated 2.1.299") {
			t.Errorf("%s: %#v", version, line)
		}
		if want == stateUnknown && (line.Code != "agent-version-unvalidated" || !strings.Contains(line.Detail, "docs/agent-contracts.md")) {
			t.Errorf("%s: %#v", version, line)
		}
	}
}

// TestPinOfADevelopmentBuildIsNotEnforced judges the pin with the release
// rule of A10: a binary without an exact release version skips the pin, as
// scan and the hook do, and only two different releases are red.
func TestPinOfADevelopmentBuildIsNotEnforced(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ installed, pinned, state string }{
		{"0.0.0-20261016050719-17da434e4e69", "0.0.0", stateUnknown},
		{"(devel)", "0.1.0", stateUnknown},
		{"0.1.0", "0.0.0", stateRed},
		{"v0.1.0", "0.1.0", stateOK},
	} {
		input := baseline(t)
		input.Receipt.Version, input.PinnedVersion = tc.installed, tc.pinned
		line := lineOf(t, input, "doctor.pin")
		if line.State != tc.state || tc.state == stateUnknown && !strings.Contains(line.Detail, "development build, pin not enforced") {
			t.Errorf("%s against %s: %#v", tc.installed, tc.pinned, line)
		}
	}
}

// TestDispatchIsTheFinalLine reports the last dispatch evidence a hook
// process left (section 5.6): when and on which OS for the installed hook
// definition, unknown when the observation is stale, undated or absent.
func TestDispatchIsTheFinalLine(t *testing.T) {
	t.Parallel()
	seen := time.Date(2026, time.October, 16, 9, 30, 0, 0, time.UTC)
	for _, tc := range []struct {
		dispatch    Dispatch
		state, code string
	}{
		{Dispatch{State: DispatchObserved, SeenAt: seen, OS: "darwin"}, stateOK, "dispatch-observed"},
		{Dispatch{State: DispatchObserved, OS: "darwin"}, stateUnknown, "dispatch-undated"},
		{Dispatch{State: DispatchStale, SeenAt: seen, OS: "linux"}, stateUnknown, "dispatch-stale"},
		{Dispatch{}, stateUnknown, "dispatch-unobserved"},
	} {
		input := baseline(t)
		input.Dispatch = tc.dispatch
		lines := Assess(input).Lines
		last := lines[len(lines)-1]
		if last.ID != "doctor.dispatch" || last.State != tc.state || last.Code != tc.code {
			t.Errorf("%#v: %#v", tc.dispatch, last)
		}
		if tc.code == "dispatch-observed" && !strings.Contains(last.Detail, "2026-10-16T09:30:00Z on darwin") {
			t.Errorf("detail = %q", last.Detail)
		}
	}
}

func TestReadDispatch(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "observation.json")
	seen := time.Date(2026, time.October, 16, 9, 30, 0, 0, time.UTC)
	observation := Observation{
		Identity: "repository", HookDigest: "definition", SeenAt: seen, OS: "windows",
		Environment: ObserveEnvironment([]string{"PATH=bin"}, "work"),
	}
	if err := SaveObservation(path, observation); err != nil {
		t.Fatal(err)
	}
	if got := ReadDispatch(path, "repository", "definition"); got.State != DispatchObserved || !got.SeenAt.Equal(seen) || got.OS != "windows" {
		t.Errorf("matching = %#v", got)
	}
	if got := ReadDispatch(path, "repository", "changed"); got.State != DispatchStale {
		t.Errorf("another definition = %#v", got)
	}
	for _, tc := range []struct{ path, identity string }{{path, "other"}, {filepath.Join(t.TempDir(), "absent.json"), "repository"}} {
		if got := ReadDispatch(tc.path, tc.identity, "definition"); got.State != DispatchUnobserved {
			t.Errorf("%s for %s = %#v", tc.path, tc.identity, got)
		}
	}
}
