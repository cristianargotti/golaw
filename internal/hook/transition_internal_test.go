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
// and a deletion keep it, and the blobs of a HEAD that moved are asked for
// once.
func TestContentChangedLeavesWhatOnlyItsStatusMoved(t *testing.T) {
	t.Parallel()
	before, after, blobs, snapshot := transition()
	run := execution{}
	want := []string{"dirty.go", "edit.go", "gone.go", "new.go"}
	same := Changes{Snapshot: snapshot, Head: "c1", Blobs: blobs}
	if got := run.contentChanged(t.Context(), Dependencies{}, before, after, same); !slices.Equal(got, want) {
		t.Fatalf("HEAD unchanged = %v, want %v", got, want)
	}
	asked := 0
	deps := Dependencies{Blobs: func(_ context.Context, _ Repository, commit string, paths []string) (map[string]string, error) {
		asked++
		if commit != "c1" || !slices.Equal(paths, []string{"edit.go", "gone.go", "mode.go", "new.go"}) {
			t.Fatalf("asked %s for %v", commit, paths)
		}
		return blobs, nil
	}}
	if got := run.contentChanged(t.Context(), deps, before, after, Changes{Snapshot: snapshot, Head: "c2"}); !slices.Equal(got, want) || asked != 1 {
		t.Fatalf("HEAD moved = %v after %d questions, want %v", got, asked, want)
	}
	if got := run.contentChanged(t.Context(), Dependencies{}, before, map[string]string{"dirty.go": Hash([]byte("one"))}, same); len(got) != 0 {
		t.Fatalf("nothing changed = %v", got)
	}
}

// TestContentChangedKeepsWhatItCannotCompare keeps every path whose
// status alone changed when the blob of the pre event cannot be known: a
// binding without blobs or failing to answer them after HEAD moved, and
// an unborn branch before the command.
func TestContentChangedKeepsWhatItCannotCompare(t *testing.T) {
	t.Parallel()
	before, after, blobs, snapshot := transition()
	run := execution{}
	all := []string{"dirty.go", "edit.go", "gone.go", "mode.go", "new.go"}
	moved := Changes{Snapshot: snapshot, Head: "c2"}
	failing := Dependencies{Blobs: func(context.Context, Repository, string, []string) (map[string]string, error) {
		return nil, model.ErrUnreadable
	}}
	for name, d := range map[string]Dependencies{"a binding without blobs": {}, "a failing binding": failing} {
		if got := run.contentChanged(t.Context(), d, before, after, moved); !slices.Equal(got, all) {
			t.Fatalf("%s = %v, want %v", name, got, all)
		}
	}
	unborn := Baseline{Files: before.Files}
	if got := run.contentChanged(t.Context(), Dependencies{}, unborn, after, Changes{Snapshot: snapshot, Blobs: blobs}); !slices.Equal(got, all) {
		t.Fatalf("an unborn branch before the command = %v, want %v", got, all)
	}
	held := run.sameBlobs(t.Context(), Dependencies{}, Changes{Snapshot: proof.NewTree(nil)}, []string{"mode.go"}, blobs)
	if bare := run.sameBlobs(t.Context(), Dependencies{}, Changes{Snapshot: snapshot}, []string{"mode.go"}, nil); held["mode.go"] || bare["mode.go"] {
		t.Fatal("a path the snapshot does not hold, or without a blob, has the same blob")
	}
}

// TestContentChangedAsksGitWhatItConverts asks git, once, for the blob the
// bytes of a path would be stored as when they are not the blob the HEAD of
// the pre event held and git may convert them on their way in: a path whose
// status moved some other way than an edit of its bytes, and an edited one
// whose bytes hold a carriage return. An edit without one is changed with
// no question, and a binding that cannot answer keeps every path changed.
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
	if got := run.contentChanged(t.Context(), deps, before, after, changes); !slices.Equal(got, []string{"edited-cr.md", "edited.md"}) ||
		len(asked) != 1 || !slices.Equal(asked[0], []string{"crlf.md", "edited-cr.md"}) {
		t.Fatalf("changed %v after asking %v", got, asked)
	}
	deps.Hashes = func(context.Context, Repository, []string) (map[string]string, error) {
		return nil, model.ErrUnreadable
	}
	if got := run.contentChanged(t.Context(), deps, before, after, changes); len(got) != 3 {
		t.Fatalf("a binding that cannot answer = %v", got)
	}
}
