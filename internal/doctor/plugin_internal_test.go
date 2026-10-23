package doctor

import (
	"encoding/json"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
)

func TestEveryHookHasItsOwnFailureLine(t *testing.T) {
	t.Parallel()
	base := baseline(t)
	hooks, err := install.ParseHooks([]byte(base.Hooks))
	if err != nil {
		t.Fatal(err)
	}
	for event, groups := range hooks.Hooks {
		for index := range groups {
			key := event + "/" + groups[index].Matcher
			if !failOpenReported(t, base, event, index, key) {
				t.Fatalf("handler not measured independently: %s", key)
			}
		}
	}
}

func failOpenReported(t *testing.T, base Snapshot, event string, index int, key string) bool {
	t.Helper()
	changed, err := install.ParseHooks([]byte(base.Hooks))
	if err != nil {
		t.Fatal(err)
	}
	changed.Hooks[event][index].Hooks[0].OnFailure = "allow"
	encoded, err := json.Marshal(changed)
	if err != nil {
		t.Fatal(err)
	}
	input := base
	input.Hooks = string(encoded)
	found := false
	for _, line := range assess(input).Lines {
		if line.ID == idHooks && line.Key == key {
			found = line.State == stateRed && line.Code == codeHookFailOpen
		}
	}
	return found
}

func TestTrustNeverBecomesVerified(t *testing.T) {
	t.Parallel()
	for _, plugin := range []string{"", `{"name":"golaw","version":"0.1.0"}`} {
		input := baseline(t)
		input.Plugin = plugin
		found := false
		for _, line := range assess(input).Lines {
			if line.ID == "doctor.trust" {
				found = line.State == "unknown"
			}
		}
		if !found {
			t.Fatal("plugin trust was omitted or asserted")
		}
	}
}
