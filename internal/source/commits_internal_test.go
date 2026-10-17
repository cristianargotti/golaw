package source

import (
	"slices"
	"testing"
)

// TestParseCommitsReadsEachShapeGitLogPrints reads the records git log -z
// prints in the format of Commits: a commit with paths after the newline
// git puts before the first, an empty commit, a merge whose paths follow an
// empty field of their own, a merge without paths before another commit
// and a root commit last; and refuses a record that opens without its two
// empty fields, one cut short and a time that is no number.
func TestParseCommitsReadsEachShapeGitLogPrints(t *testing.T) {
	t.Parallel()
	out := "\x00\x00c1\x00100\x00me\x00me@x\x00p1\x00\na.md\x00b c.md\x00" +
		"\x00\x00c2\x00101\x00me\x00me@x\x00p2\x00" +
		"\x00\x00m1\x00102\x00me\x00me@x\x00p3 p4\x00\x00mine.md\x00shared.md\x00" +
		"\x00\x00m2\x00103\x00mate\x00mate@x\x00p5 p6\x00\x00" +
		"\x00\x00r1\x00104\x00mate\x00mate@x\x00\x00\nroot.md\x00"
	commits, err := parseCommits(out)
	if err != nil || len(commits) != 5 {
		t.Fatalf("commits = %+v, %v", commits, err)
	}
	want := []Commit{
		{Name: "c1", Committer: "me <me@x>", Time: 100, Paths: []string{"a.md", "b c.md"}},
		{Name: "c2", Committer: "me <me@x>", Time: 101},
		{Name: "m1", Committer: "me <me@x>", Time: 102, Merge: true, Paths: []string{"mine.md", "shared.md"}},
		{Name: "m2", Committer: "mate <mate@x>", Time: 103, Merge: true},
		{Name: "r1", Committer: "mate <mate@x>", Time: 104, Paths: []string{"root.md"}},
	}
	for i, commit := range commits {
		if commit.Name != want[i].Name || commit.Committer != want[i].Committer || commit.Time != want[i].Time ||
			commit.Merge != want[i].Merge || !slices.Equal(commit.Paths, want[i].Paths) {
			t.Fatalf("commit %d = %+v, want %+v", i, commit, want[i])
		}
	}
	for name, bad := range map[string]string{
		"a record without its two empty fields": "\x00c1\x00100\x00me\x00me@x\x00p1\x00",
		"a record cut short":                    "\x00\x00c1\x00100\x00me",
		"a time that is no number":              "\x00\x00c1\x00soon\x00me\x00me@x\x00p1\x00",
	} {
		if got, err := parseCommits(bad); err == nil {
			t.Fatalf("%s answered %+v", name, got)
		}
	}
	if none, err := parseCommits(""); err != nil || len(none) != 0 {
		t.Fatalf("no output = %+v, %v", none, err)
	}
}
