package shell

import (
	"slices"
	"strings"
	"testing"
)

// TestCmdLaunchedAnswersEachTailOnce bounds the commands the start and call
// words of a cmd command run by the words themselves: every such command is
// the tail of the words from some index, so a run of n start words before a
// program answers its n tails and the title reading of the last start once
// each, instead of doubling at every start with both readings of a title.
func TestCmdLaunchedAnswersEachTailOnce(t *testing.T) {
	t.Parallel()
	for _, n := range []int{1, 2, 3, 10, 25} {
		words := append(strings.Fields(strings.Repeat("start ", n)), "git", "status")
		got := cmdLaunched(words)
		if len(got) != n+1 {
			t.Errorf("%d start words: %d commands, want %d", n, len(got), n+1)
			continue
		}
		for at := 1; at <= n+1; at++ {
			if !slices.ContainsFunc(got, func(run []string) bool { return slices.Equal(run, words[at:]) }) {
				t.Errorf("%d start words: the tail from %d is missing", n, at)
			}
		}
	}
	mixed := append(strings.Fields(strings.Repeat("call start /b ", 20)), "git", "status")
	if got := cmdLaunched(mixed); len(got) > len(mixed) {
		t.Errorf("call and start mixed: %d commands for %d words", len(got), len(mixed))
	}
}
