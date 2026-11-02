package main

import (
	"strings"
	"testing"
)

func TestRootCommandRegistersMap(t *testing.T) {
	t.Parallel()
	ta := newTestApp(t)
	cmd, _, err := ta.rootCommand().Find([]string{"map"})
	if err != nil || cmd.Name() != "map" {
		t.Fatalf("Find(map) = %v, %v", cmd, err)
	}
	if code := ta.run(t.Context(), []string{"map", "--help"}); code != exitClean {
		t.Fatalf("map --help exit = %d: %s", code, ta.stderr.String())
	}
	for _, flag := range []string{"--budget", "--area", "--json"} {
		if !strings.Contains(ta.stdout.String(), flag) {
			t.Errorf("map --help lacks %s: %s", flag, ta.stdout.String())
		}
	}
}
