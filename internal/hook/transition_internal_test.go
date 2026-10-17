package hook

import (
	"context"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// The names git gives the blob of "hello\n" in each object format.
const (
	helloSHA1   = "ce013625030ba8dba906f756967f9e9ca394464a"
	helloSHA256 = "2cf8d83d9ee29543b34a87727421fdecb7e3f3a183d337639025de576db9ebb4"
)

func TestBlobNameIsTheNameGitGives(t *testing.T) {
	t.Parallel()
	hello := []byte("hello\n")
	if got := blobName(hello, len(helloSHA1)); got != helloSHA1 {
		t.Errorf("sha1 name = %s", got)
	}
	if got := blobName(hello, len(helloSHA256)); got != helloSHA256 {
		t.Errorf("sha256 name = %s", got)
	}
	if got := blobName(hello, 7); got != "" {
		t.Errorf("a name of another length = %q", got)
	}
}

// transition is the identities of a pre and a post event around a command
// that changed the status of five paths: a file dirty before, a mode
// change of a committed file, an edit, a new file and a deletion.
func transition() (Baseline, map[string]string, map[string]string, model.Snapshot) {
	snapshot := proof.NewTree(map[string][]byte{"mode.go": []byte("hello\n"), "edit.go": []byte("changed\n"), "new.go": []byte("hello\n")})
	before := Baseline{Files: map[string]string{"dirty.go": Hash([]byte("one"))}, Head: "c1"}
	after := map[string]string{
		"dirty.go": Hash([]byte("two")), "mode.go": Hash([]byte("hello\n")), "edit.go": Hash([]byte("changed\n")),
		"new.go": Hash([]byte("hello\n")), "gone.go": absentIdentity,
	}
	blobs := map[string]string{"mode.go": helloSHA1, "edit.go": helloSHA1, "gone.go": helloSHA1}
	return before, after, blobs, snapshot
}

// TestContentChangedLeavesWhatOnlyItsStatusMoved compares a path git
// status named after the command and not before it with the blob the HEAD
// of the pre event held: its own bytes leave it out, other bytes, no blob
// and a deletion keep it, and after a move of HEAD the blob the earlier
// commit held is the one the move answers, the blob HEAD holds now where
// the move changed nothing.
func TestContentChangedLeavesWhatOnlyItsStatusMoved(t *testing.T) {
	t.Parallel()
	before, after, blobs, snapshot := transition()
	run := execution{}
	want := []string{"dirty.go", "edit.go", "gone.go", "new.go"}
	same := Changes{Snapshot: snapshot, Head: "c1", Blobs: blobs}
	if got, gaps := run.contentChanged(t.Context(), Dependencies{}, before, after, same, moves{known: true}); !slices.Equal(got, want) || len(gaps) != 0 {
		t.Fatalf("HEAD unchanged = %v, %v, want %v", got, gaps, want)
	}
	named := []string{"dirty.go", "edit.go", "gone.go", "mode.go", "new.go"}
	moved := moves{paths: map[string]string{"edit.go": helloSHA1, "gone.go": helloSHA1}, known: true}
	now := Changes{Snapshot: snapshot, Head: "c2", Paths: named, Blobs: map[string]string{"mode.go": helloSHA1, "edit.go": "other"}}
	if got, gaps := run.contentChanged(t.Context(), Dependencies{}, before, after, now, moved); !slices.Equal(got, want) || len(gaps) != 0 {
		t.Fatalf("HEAD moved = %v, %v, want %v", got, gaps, want)
	}
	if got, gaps := run.contentChanged(t.Context(), Dependencies{}, before, map[string]string{"dirty.go": Hash([]byte("one"))}, same, moves{known: true}); len(got) != 0 || len(gaps) != 0 {
		t.Fatalf("nothing changed = %v, %v", got, gaps)
	}
}

// TestContentChangedJudgesWhatAMoveOfHeadChanged keeps a path the move of
// HEAD changed that git status reports clean after it, without asking git
// how it stores the bytes, and leaves one whose bytes are still the blob
// the earlier commit held.
func TestContentChangedJudgesWhatAMoveOfHeadChanged(t *testing.T) {
	t.Parallel()
	snapshot := proof.NewTree(map[string][]byte{"committed.go": []byte("changed\n"), "back.go": []byte("hello\n")})
	after := map[string]string{"committed.go": Hash([]byte("changed\n")), "back.go": Hash([]byte("hello\n"))}
	moved := moves{paths: map[string]string{"committed.go": helloSHA1, "back.go": helloSHA1}, known: true}
	asked := 0
	deps := Dependencies{Hashes: func(context.Context, Repository, []string) (map[string]string, error) {
		asked++
		return nil, model.ErrUnreadable
	}}
	before, run := Baseline{Files: map[string]string{}, Head: "c1"}, execution{}
	if got, gaps := run.contentChanged(t.Context(), deps, before, after, Changes{Snapshot: snapshot, Head: "c2"}, moved); !slices.Equal(got, []string{"committed.go"}) || len(gaps) != 0 || asked != 0 {
		t.Fatalf("changed %v, %v after %d questions", got, gaps, asked)
	}
}

// TestContentChangedNamesWhatItCannotCompare judges what it compares
// without the blob of the pre event and names what it cannot: after a move
// of HEAD no binding could read, a path named before the command is judged
// by its bytes and every path whose status alone changed is a gap, never a
// change; on an unborn branch before the command, whose tree is empty,
// every path named after it only had no blob, so each is changed.
func TestContentChangedNamesWhatItCannotCompare(t *testing.T) {
	t.Parallel()
	before, after, blobs, snapshot := transition()
	run := execution{}
	all := []string{"dirty.go", "edit.go", "gone.go", "mode.go", "new.go"}
	got, gaps := run.contentChanged(t.Context(), Dependencies{}, before, after, Changes{Snapshot: snapshot, Head: "c2", Blobs: blobs}, moves{})
	named := make([]string, 0, len(gaps))
	for _, gap := range gaps {
		if gap.Cause != causeMove {
			t.Fatalf("an unknown move named %+v", gap)
		}
		named = append(named, gap.Path)
	}
	if !slices.Equal(got, []string{"dirty.go"}) || !slices.Equal(named, all[1:]) {
		t.Fatalf("an unknown move = %v, gaps %v", got, gaps)
	}
	unborn := Baseline{Files: before.Files}
	if got, gaps := run.contentChanged(t.Context(), Dependencies{}, unborn, after, Changes{Snapshot: snapshot, Blobs: blobs}, moves{known: true}); !slices.Equal(got, all) || len(gaps) != 0 {
		t.Fatalf("an unborn branch before the command = %v, %v, want %v", got, gaps, all)
	}
	held, _ := run.sameBlobs(t.Context(), Dependencies{}, Changes{Snapshot: proof.NewTree(nil)}, []string{"mode.go"}, blobs, nil)
	if bare, _ := run.sameBlobs(t.Context(), Dependencies{}, Changes{Snapshot: snapshot}, []string{"mode.go"}, nil, nil); held["mode.go"] || bare["mode.go"] {
		t.Fatal("a path the snapshot does not hold, or without a blob, has the same blob")
	}
}

// TestMovedSinceAsksOnlyWhenHeadMoved asks the binding for the move of HEAD
// once, only when the HEAD of the post event is another commit than the
// one of the pre event, an unborn branch at either end included, which the
// binding reads as the empty tree, and answers an unknown move without a
// binding and when the binding fails.
func TestMovedSinceAsksOnlyWhenHeadMoved(t *testing.T) {
	t.Parallel()
	asked := 0
	deps := Dependencies{Moved: func(_ context.Context, _ Repository, from, to string) (map[string]string, error) {
		asked++
		if from == "c0" {
			return nil, model.ErrUnreadable
		}
		return map[string]string{"a.go": from + "|" + to}, nil
	}}
	run := execution{repository: Repository{Head: "c2"}}
	if got := run.movedSince(t.Context(), deps, "c2"); !got.known || got.paths != nil || asked != 0 {
		t.Fatalf("HEAD in place = %+v after %d questions", got, asked)
	}
	if got := run.movedSince(t.Context(), deps, "c1"); !got.known || got.paths["a.go"] != "c1|c2" || asked != 1 {
		t.Fatalf("HEAD moved = %+v after %d questions", got, asked)
	}
	if got := run.movedSince(t.Context(), deps, ""); !got.known || got.paths["a.go"] != "|c2" || asked != 2 {
		t.Fatalf("from an unborn branch = %+v after %d questions", got, asked)
	}
	orphan := execution{repository: Repository{}}
	if got := orphan.movedSince(t.Context(), deps, "c1"); !got.known || got.paths["a.go"] != "c1|" || asked != 3 {
		t.Fatalf("to an unborn branch = %+v after %d questions", got, asked)
	}
	if got := orphan.movedSince(t.Context(), deps, ""); !got.known || asked != 3 {
		t.Fatalf("an unborn branch at both ends = %+v after %d questions", got, asked)
	}
	if got := run.movedSince(t.Context(), deps, "c0"); got.known {
		t.Fatalf("a failing binding = %+v", got)
	}
	if got := run.movedSince(t.Context(), Dependencies{}, "c1"); got.known {
		t.Fatalf("no binding = %+v", got)
	}
}

// TestContentChangedAsksGitWhatItConverts asks git, once, for the blob the
// bytes of a path would be stored as when they are not the blob the HEAD of
// the pre event held and git may convert them on their way in: a path whose
// status moved some other way than an edit of its bytes, and an edited one
// whose bytes hold a carriage return. An edit without one is changed with
// no question; when the binding cannot answer, or names no blob for a path,
// each path it was asked is a gap that names it and never a change, while
// the edit found changed without the question stays changed.
func TestContentChangedAsksGitWhatItConverts(t *testing.T) {
	t.Parallel()
	snapshot := proof.NewTree(map[string][]byte{"crlf.md": []byte("hello\r\n"), "edited.md": []byte("changed\n"), "edited-cr.md": []byte("x\r\n")})
	after := map[string]string{"crlf.md": "c", "edited.md": "e", "edited-cr.md": "r"}
	blobs := map[string]string{"crlf.md": helloSHA1, "edited.md": helloSHA1, "edited-cr.md": helloSHA1}
	changes := Changes{Snapshot: snapshot, Head: "c1", Blobs: blobs, Edited: map[string]bool{"edited.md": true, "edited-cr.md": true}}
	before := Baseline{Files: map[string]string{}, Head: "c1"}
	var asked [][]string
	deps := Dependencies{Hashes: func(_ context.Context, _ Repository, paths []string) (map[string]string, error) {
		asked = append(asked, paths)
		return map[string]string{"crlf.md": helloSHA1, "edited-cr.md": "other"}, nil
	}}
	run := execution{}
	if got, _ := run.contentChanged(t.Context(), deps, before, after, changes, moves{known: true}); !slices.Equal(got, []string{"edited-cr.md", "edited.md"}) ||
		len(asked) != 1 || !slices.Equal(asked[0], []string{"crlf.md", "edited-cr.md"}) {
		t.Fatalf("changed %v after asking %v", got, asked)
	}
	deps.Hashes = func(context.Context, Repository, []string) (map[string]string, error) {
		return nil, model.ErrUnreadable
	}
	got, gaps := run.contentChanged(t.Context(), deps, before, after, changes, moves{known: true})
	if !slices.Equal(got, []string{"edited.md"}) || !slices.Equal(gaps, pathGaps([]string{"crlf.md", "edited-cr.md"}, causeConvert)) {
		t.Fatalf("a binding that cannot answer = %v, %+v", got, gaps)
	}
	deps.Hashes = func(context.Context, Repository, []string) (map[string]string, error) {
		return map[string]string{"crlf.md": helloSHA1}, nil
	}
	got, gaps = run.contentChanged(t.Context(), deps, before, after, changes, moves{known: true})
	if !slices.Equal(got, []string{"edited.md"}) || !slices.Equal(gaps, pathGaps([]string{"edited-cr.md"}, causeConvert)) {
		t.Fatalf("a binding that names no blob for a path = %v, %+v", got, gaps)
	}
}
