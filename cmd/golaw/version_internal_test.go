package main

import (
	"strings"
	"testing"
)

func TestVersionPrintsMetadataAndTheSourceBuildState(t *testing.T) {
	t.Parallel()
	ta := newTestApp(t)
	if got := ta.run(t.Context(), []string{"version"}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	want := "golaw 9.9.9\nmodule: example.test/kit\nbuild: source build, executable not verifiable\n"
	if got := ta.stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestVersionRefusesArguments(t *testing.T) {
	t.Parallel()
	ta := newTestApp(t)
	if got := ta.run(t.Context(), []string{"version", "extra"}); got != exitFailure {
		t.Fatalf("exit = %d", got)
	}
	if !strings.Contains(ta.stderr.String(), `unknown command "extra"`) {
		t.Fatalf("stderr = %q", ta.stderr.String())
	}
}
