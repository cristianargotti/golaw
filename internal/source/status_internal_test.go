package source

import (
	"errors"
	"slices"
	"testing"
)

// TestParseStatusReadsEveryRecord reads the records git status
// --porcelain=v2 -z prints: an untracked file and a nested repository, an
// ordinary change with a space in its path, both paths of a rename, an
// unmerged path and a deleted submodule, while a header and a submodule
// whose only change is its revision name nothing.
func TestParseStatusReadsEveryRecord(t *testing.T) {
	t.Parallel()
	out := []byte("# stash 1\x00" +
		"? new file.md\x00? nested/\x00" +
		"1 .M N... 100644 100644 100644 a1 a2 sp ace.md\x00" +
		"2 R. N... 100644 100644 100644 b1 b2 R100 to.md\x00from.md\x00" +
		"u UU N... 100644 100644 100644 100644 c1 c2 c3 both.md\x00" +
		"1 .M SC.. 160000 160000 160000 d1 d2 moved\x00" +
		"1 .D S... 160000 160000 000000 e1 e2 gone\x00")
	entries, err := parseStatus(out)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		paths = append(paths, e.path)
		if e.path == "nested" && (!e.dir || e.tracked) || e.path == "sp ace.md" && !e.tracked {
			t.Fatalf("entry %+v", e)
		}
	}
	if want := []string{"both.md", "from.md", "gone", "nested", "new file.md", "sp ace.md", "to.md"}; !slices.Equal(paths, want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
	for _, bad := range []string{"1 .M N... short", "x what\x00"} {
		if _, err := parseStatus([]byte(bad)); !errors.Is(err, ErrGit) {
			t.Errorf("%q = %v, want ErrGit", bad, err)
		}
	}
}

// TestParseStatusKeepsTheBlobHeadHolds keeps the object HEAD holds at the
// path of an ordinary record and at the original path of a rename, never
// at the path a rename leads to, an unmerged path or an untracked one, and
// merges the records of one path into one entry.
func TestParseStatusKeepsTheBlobHeadHolds(t *testing.T) {
	t.Parallel()
	entries, err := parseStatus([]byte("? new file.md\x00" +
		"1 .M N... 100644 100644 100755 a1 a2 sp ace.md\x00" +
		"2 R. N... 100644 100644 100644 b1 b2 R100 to.md\x00from.md\x00" +
		"u UU N... 100644 100644 100644 100644 c1 c2 c3 both.md\x00"))
	if err != nil {
		t.Fatal(err)
	}
	heads := map[string]string{}
	for _, e := range entries {
		heads[e.path] = e.head
	}
	if heads["sp ace.md"] != "a1" || heads["from.md"] != "b1" || heads["to.md"] != "" || heads["both.md"] != "" || heads["new file.md"] != "" {
		t.Fatalf("the blobs HEAD holds = %v", heads)
	}
	added, err := parseStatus([]byte("1 A. N... 000000 100644 100644 0000000000 f2 added.md\x00? added.md\x00"))
	if err != nil || len(added) != 1 || added[0].head != "" || !added[0].tracked {
		t.Fatalf("an added file with its untracked twin = %+v, %v", added, err)
	}
}

// TestParseStatusNamesWhatGitFoundEdited names a path whose only change is
// an edit of its bytes in the worktree under its own mode, which git found
// by comparing them with the index, and no path whose mode, index entry or
// presence moved instead.
func TestParseStatusNamesWhatGitFoundEdited(t *testing.T) {
	t.Parallel()
	entries, err := parseStatus([]byte("1 .M N... 100644 100644 100644 a1 a2 edited.md\x00" +
		"1 .M N... 100644 100644 100755 b1 b2 mode.md\x00" +
		"1 M. N... 100644 100644 100644 c1 c2 staged.md\x00" +
		"1 MM N... 100644 100644 100644 d1 d2 both.md\x00" +
		"1 D. N... 100644 000000 000000 e1 0000 removed.md\x00? removed.md\x00"))
	if err != nil {
		t.Fatal(err)
	}
	var edited []string
	for _, e := range entries {
		if e.edited {
			edited = append(edited, e.path)
		}
	}
	if !slices.Equal(edited, []string{"edited.md"}) {
		t.Fatalf("edited = %v", edited)
	}
}

// TestParseStatusNamesStagedAndUnmergedPaths names a path whose index entry
// differs from HEAD while its bytes on disk are that entry, as a merge
// leaves what it staged, and a path git left unmerged, and neither a path
// edited again after it was staged nor one whose only change is in the
// worktree.
func TestParseStatusNamesStagedAndUnmergedPaths(t *testing.T) {
	t.Parallel()
	entries, err := parseStatus([]byte("1 A. N... 000000 100644 100644 0000 a2 added.md\x00" +
		"1 AM N... 000000 100644 100644 0000 b2 rewritten.md\x00" +
		"1 M. N... 100644 100644 100644 c1 c2 staged.md\x00" +
		"1 .M N... 100644 100644 100644 d1 d2 edited.md\x00" +
		"2 R. N... 100644 100644 100644 e1 e2 R100 to.md\x00from.md\x00" +
		"u UU N... 100644 100644 100644 100644 f1 f2 f3 both.md\x00"))
	if err != nil {
		t.Fatal(err)
	}
	var staged, unmerged []string
	for _, e := range entries {
		if e.staged {
			staged = append(staged, e.path)
		}
		if e.unmerged {
			unmerged = append(unmerged, e.path)
		}
	}
	if !slices.Equal(staged, []string{"added.md", "staged.md", "to.md"}) || !slices.Equal(unmerged, []string{"both.md"}) {
		t.Fatalf("staged %v, unmerged %v", staged, unmerged)
	}
}
