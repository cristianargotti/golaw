package rung

import (
	"slices"
	"testing"
)

func TestEmbedFieldsUnquoteAndDropTheAllPrefix(t *testing.T) {
	t.Parallel()
	got := embedFields(" assets/*.txt \"my file.txt\"\t`raw file.txt` all:static \"say \\\"hi\\\".txt\"")
	if want := []string{"assets/*.txt", "my file.txt", "raw file.txt", "static", `say "hi".txt`}; !slices.Equal(got, want) {
		t.Errorf("embedFields = %q, want %q", got, want)
	}
	if got := embedFields(" \"open.txt"); !slices.Equal(got, []string{`"open.txt`}) {
		t.Errorf("an unclosed quote = %q", got)
	}
}

func TestEmbedMatchesAFileOrADirectoryAboveIt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		pattern, rel string
		want         bool
	}{
		{"assets/message.txt", "assets/message.txt", true},
		{"assets", "assets/deep/message.txt", true},
		{"assets/*.txt", "assets/message.txt", true},
		{"static", "assets/message.txt", false},
		{"assets/*.md", "assets/message.txt", false},
		{"[", "assets/message.txt", true},
	} {
		if got := embedMatches(tc.pattern, tc.rel); got != tc.want {
			t.Errorf("embedMatches(%q, %q) = %v, want %v", tc.pattern, tc.rel, got, tc.want)
		}
	}
}
