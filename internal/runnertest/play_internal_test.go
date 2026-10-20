package runnertest

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The fake answers in the child process the tests start, which the cover
// profile of the parent never sees, so play is also proven in process.

func writeBehavior(t *testing.T, base, text string) {
	t.Helper()
	if err := os.WriteFile(base+behaviorSuffix, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestPlayAnswersTheProbeAndEveryOtherCall(t *testing.T) {
	t.Parallel()
	base := filepath.Join(t.TempDir(), "govulncheck")
	writeBehavior(t, base, `{"probe":["-version"],"version":"Scanner: govulncheck@v1.7.0","stdout":"{}\n","stderr":"db\n","code":3}`)
	var stdout, stderr bytes.Buffer
	if code, fake := play(base+".exe", []string{"-version"}, &stdout, &stderr); !fake || code != 0 || stdout.String() != "Scanner: govulncheck@v1.7.0\n" {
		t.Errorf("probe = %d, %v, %q", code, fake, stdout.String())
	}
	stdout.Reset()
	if code, fake := play(base, []string{"-format", "json", "./..."}, &stdout, &stderr); !fake || code != 3 || stdout.String() != "{}\n" || stderr.String() != "db\n" {
		t.Errorf("scan = %d, %v, %q, %q", code, fake, stdout.String(), stderr.String())
	}
	calls := readCalls(t, base)
	if len(calls) != 2 || !slices.Equal(calls[1], []string{"-format", "json", "./..."}) {
		t.Errorf("calls = %v", calls)
	}
}

func TestPlayIsNoFakeWithoutABehavior(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if _, fake := play(filepath.Join(t.TempDir(), "golaw.test"), nil, &stdout, &stderr); fake || stdout.Len() != 0 {
		t.Error("a binary without a behavior beside it runs its tests")
	}
}

func TestPlayRefusesABrokenBehaviorOrRecord(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	broken := filepath.Join(dir, "broken")
	writeBehavior(t, broken, "{")
	var stdout, stderr bytes.Buffer
	if code, fake := play(broken, nil, &stdout, &stderr); !fake || code != BrokenCode || !strings.Contains(stderr.String(), "behavior") {
		t.Errorf("broken behavior = %d, %v, %q", code, fake, stderr.String())
	}
	unrecorded := filepath.Join(dir, "unrecorded")
	writeBehavior(t, unrecorded, "{}")
	if err := os.Mkdir(unrecorded+callsSuffix, 0o750); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if code, fake := play(unrecorded, nil, &stdout, &stderr); !fake || code != BrokenCode || !strings.Contains(stderr.String(), "record") {
		t.Errorf("unrecorded call = %d, %v, %q", code, fake, stderr.String())
	}
}
