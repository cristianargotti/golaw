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

// Main reads the person's race options through the getenv its TestMain
// hands it, so the helper itself never reads the environment.
func TestQuickRaceKeepsWhatTheGetenvAnswers(t *testing.T) {
	t.Setenv("GORACE", "")
	asked := ""
	getenv := func(name string) string { asked = name; return "halt_on_error=1" }
	if err := quickRace(getenv); err != nil {
		t.Fatalf("quickRace: %v", err)
	}
	if asked != "GORACE" {
		t.Errorf("quickRace asked for %q, want GORACE", asked)
	}
	if got, want := os.Getenv("GORACE"), "halt_on_error=1 atexit_sleep_ms=0"; got != want {
		t.Errorf("GORACE = %q, want %q", got, want)
	}
}

// A value the system refuses to hold comes back as the error Main reports.
func TestQuickRaceAnswersTheRefusedOptions(t *testing.T) {
	t.Setenv("GORACE", "")
	if err := quickRace(func(string) string { return "a=1\x00" }); err == nil || !strings.Contains(err.Error(), "set GORACE") {
		t.Errorf("quickRace = %v, want the refused set of GORACE", err)
	}
}
