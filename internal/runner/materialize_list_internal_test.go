package runner

import (
	"errors"
	"io"
	"runtime"
	"slices"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

func TestTreeEntriesRequireCompleteBoundedRecords(t *testing.T) {
	t.Parallel()
	object := strings.Repeat("a", 40)
	first := "100644 " + object + " 0\ta\x00"
	second := "160000 " + object + " 0\tb\x00"
	for _, tc := range []struct {
		name, stream string
		limit        int
		want         error
		count        int
	}{
		{"empty", "", 1, nil, 0},
		{"exact", first + second, 2, nil, 2},
		{"file-count", first + second, 1, ErrBound, 0},
		{"truncated", strings.TrimSuffix(first, "\x00"), 1, model.ErrCannotJudge, 0},
		{"malformed", "invalid\x00", 1, model.ErrCannotJudge, 0},
		{"unmerged", "100644 " + object + " 2\ta\x00", 1, model.ErrCannotJudge, 0},
		{"duplicate", first + first, 2, model.ErrCannotJudge, 0},
		{"prefix-collision", first + "100644 " + object + " 0\ta/b\x00", 2, model.ErrCannotJudge, 0},
		{"oversized-record", strings.Repeat("x", (128<<10)+1) + "\x00", 1, model.ErrCannotJudge, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			entries, err := readTreeEntries(strings.NewReader(tc.stream), false, tc.limit)
			if !errors.Is(err, tc.want) || (err == nil && len(entries) != tc.count) {
				t.Fatalf("entries=%+v error=%v, want %d entries and %v", entries, err, tc.count, tc.want)
			}
			if errors.Is(tc.want, ErrBound) && !strings.Contains(err.Error(), "2") {
				t.Fatalf("observed file count missing: %v", err)
			}
		})
	}
	_, err := readTreeEntries(iotest.ErrReader(io.ErrClosedPipe), false, 1)
	if !errors.Is(err, model.ErrCannotJudge) || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("listing reader error = %v", err)
	}
}

func TestTreeEntryChecksIndexAndRevisionMetadata(t *testing.T) {
	t.Parallel()
	object := strings.Repeat("a", 40)
	for _, tc := range []struct {
		name, record, mode string
		revision, bad      bool
	}{
		{"index", "100644 " + object + " 0\tfile", "100644", false, false},
		{"executable", "100755 " + object + " 0\tfile", "100755", false, false},
		{"link", "120000 " + object + " 0\tfile", "120000", false, false},
		{"index-gitlink", "160000 " + object + " 0\tfile", "160000", false, false},
		{"revision-blob", "100644 blob " + object + "\tfile", "100644", true, false},
		{"revision-gitlink", "160000 commit " + object + "\tfile", "160000", true, false},
		{"revision-gitlink-blob", "160000 blob " + object + "\tfile", "", true, true},
		{"revision-commit-file", "100644 commit " + object + "\tfile", "", true, true},
		{"revision-tree", "040000 tree " + object + "\tfile", "", true, true},
		{"missing-tab", "100644 " + object + " 0 file", "", false, true},
		{"missing-field", "100644 " + object + "\tfile", "", false, true},
		{"extra-field", "100644 " + object + " 0 extra\tfile", "", false, true},
		{"bad-object", "100644 invalid 0\tfile", "", false, true},
		{"zero-object", "100644 " + strings.Repeat("0", 40) + " 0\tfile", "", false, true},
		{"bad-mode", "100600 " + object + " 0\tfile", "", false, true},
		{"unmerged", "100644 " + object + " 1\tfile", "", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			entry, err := parseTreeEntry(tc.record, tc.revision)
			if tc.bad {
				if !errors.Is(err, model.ErrCannotJudge) {
					t.Fatalf("entry=%+v error=%v", entry, err)
				}
				return
			}
			if err != nil || entry.path != "file" || entry.mode != tc.mode || entry.object != object {
				t.Fatalf("entry=%+v error=%v", entry, err)
			}
		})
	}
}

func TestTreePathsRejectAliases(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"", ".", "..", "../file", "/file", "\\root", "a/../b", "a/./b", "a//b", "a/",
		".git/config", ".GIT", ".git./config",
		".git /config", "git~1/config", "GIT~1/config", "dir/.git/config",
	} {
		t.Run("reject-"+name, func(t *testing.T) {
			t.Parallel()
			if err := validateTreePath(name); !errors.Is(err, model.ErrCannotJudge) {
				t.Fatalf("unsafe path %q accepted: %v", name, err)
			}
		})
	}
}

// TestTreePathsJudgeAColonByTheHost pins that a colon is a drive or a
// stream only where Windows reads it so: there the path is refused, and
// elsewhere a name such as a timestamp keeps its colon.
func TestTreePathsJudgeAColonByTheHost(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"C:relative", "C:/rooted", "dir/name:stream", "2026-10-20T10:00:00Z.json"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := validateTreePath(name)
			if runtime.GOOS == "windows" {
				if !errors.Is(err, model.ErrCannotJudge) {
					t.Fatalf("windows accepted %q: %v", name, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("colon refused on %s: %v", runtime.GOOS, err)
			}
		})
	}
}

func TestTreeEntriesPreserveNames(t *testing.T) {
	t.Parallel()
	object := strings.Repeat("a", 40)
	for _, name := range []string{
		"space name.txt", "tab\tname.txt", "line\nname.txt", "música/ação.txt", " leading.txt",
		".gitignore", "dir/.github/config", "git~10/config", "directory/file.txt",
	} {
		t.Run("preserve-"+name, func(t *testing.T) {
			t.Parallel()
			for _, revision := range []bool{false, true} {
				header := "100644 " + object + " 0\t"
				if revision {
					header = "100644 blob " + object + "\t"
				}
				entries, err := readTreeEntries(strings.NewReader(header+name+"\x00"), revision, 1)
				if err != nil || len(entries) != 1 || entries[0].path != name || entries[0].object != object {
					t.Fatalf("preserved path %q: %+v, %v", name, entries, err)
				}
			}
		})
	}
}

func TestTreeHierarchyDistinguishesParentsFromNamePrefixes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		paths []string
		bad   bool
	}{
		{"siblings", []string{"a/b", "a/c", "a/bb/c"}, false},
		{"name-prefix", []string{"a", "ab/c"}, false},
		{"duplicate", []string{"a/b", "a/b"}, true},
		{"parent-first", []string{"a", "a/b"}, true},
		{"child-first", []string{"a/b/c", "a/b"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			entries := make([]treeEntry, 0, len(tc.paths))
			for _, name := range tc.paths {
				entries = append(entries, treeEntry{path: name})
			}
			err := validateTreeHierarchy(entries)
			if (tc.bad && !errors.Is(err, model.ErrCannotJudge)) || (!tc.bad && err != nil) {
				t.Fatalf("hierarchy=%v error=%v", tc.paths, err)
			}
		})
	}
}

func TestTreeListArgsFreezeTheResolvedRevision(t *testing.T) {
	object := strings.Repeat("a", 40)
	dir := t.TempDir()
	runnertest.Install(t, dir, "git", runnertest.Behavior{Stdout: object + "\n"})
	t.Setenv("PATH", dir)
	args, err := treeListArgs(t.Context(), MaterializeInput{Rev: "--revision"})
	if err != nil || !slices.Equal(args, []string{"ls-tree", "-r", "-z", object}) {
		t.Fatalf("frozen revision args=%v error=%v", args, err)
	}
	want := [][]string{{"-c", "core.quotePath=false", "rev-parse", "--verify", "--quiet", "--end-of-options", "--revision^{tree}"}}
	if calls := runnertest.Calls(t, dir, "git"); !slices.EqualFunc(calls, want, slices.Equal) {
		t.Fatalf("revision calls = %v", calls)
	}
}

func TestTreeListArgsRejectUnusableResolutions(t *testing.T) {
	for _, tc := range []struct {
		name, output string
		code         int
	}{
		{"missing", "", 1},
		{"invalid-object", "invalid\n", 0},
		{"failed-command", "", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			runnertest.Install(t, dir, "git", runnertest.Behavior{Stdout: tc.output, Code: tc.code})
			t.Setenv("PATH", dir)
			args, err := treeListArgs(t.Context(), MaterializeInput{Rev: "--revision"})
			if err == nil {
				t.Fatalf("invalid resolution accepted: %v", args)
			}
		})
	}
}

func TestTreeListingPreservesExecutionFailures(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, revision := range []string{"", "HEAD"} {
		_, err := listTree(t.Context(), MaterializeInput{Rev: revision, Bounds: Bounds{Files: 1}})
		if !errors.Is(err, ErrStart) {
			t.Fatalf("revision %q startup error = %v", revision, err)
		}
	}
	dir := t.TempDir()
	runnertest.Install(t, dir, "git", runnertest.Behavior{Code: 2, Stderr: "cannot list"})
	t.Setenv("PATH", dir)
	if _, err := listTree(t.Context(), MaterializeInput{Bounds: Bounds{Files: 1}}); err == nil {
		t.Fatal("nonzero listing status accepted")
	}
}
