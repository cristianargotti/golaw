package runnertest

import (
	"os"
	"strings"
	"testing"
)

func TestRaceOptionsDropTheExitSleepAPersonDidNotSet(t *testing.T) {
	t.Parallel()
	for current, want := range map[string]string{
		"":                       "atexit_sleep_ms=0",
		"halt_on_error=1":        "halt_on_error=1 atexit_sleep_ms=0",
		" log_path=race.log ":    "log_path=race.log atexit_sleep_ms=0",
		"atexit_sleep_ms=500":    "atexit_sleep_ms=500",
		"a=1 atexit_sleep_ms=20": "a=1 atexit_sleep_ms=20",
	} {
		if got := raceOptions(current); got != want {
			t.Errorf("raceOptions(%q) = %q, want %q", current, got, want)
		}
	}
}

// The TestMain of this package runs Main, so the tests see what Main left
// for every process a test starts.
func TestMainLeavesTheRaceOptionsToEveryChild(t *testing.T) {
	t.Parallel()
	if got := os.Getenv("GORACE"); !strings.Contains(got, "atexit_sleep_ms=") {
		t.Errorf("GORACE = %q, want the exit sleep named, so a fake started under -race exits at once", got)
	}
}
