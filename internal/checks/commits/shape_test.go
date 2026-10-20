package commits_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/commits"
	"github.com/cristianargotti/golaw/internal/checks/shell"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// added is a commit that adds n files of one line each, the commit input
// the proof builds for a commit check fixture.
func added(t *testing.T, subject string, n int) model.Input {
	t.Helper()
	files := map[string][]byte{}
	for i := range n {
		files[fmt.Sprintf("f%02d.txt", i+1)] = []byte("line\n")
	}
	fixture := proof.Fixture{Files: files, Commit: &model.Commit{Message: subject + "\n"}}
	in, err := fixture.Input(model.ScopeCommit, proof.Policy())
	if err != nil {
		t.Fatal(err)
	}
	return in
}

// staged is a snapshot of the staged files that also holds the blobs the
// base held, by id, as the index snapshot does, and answers the classes and
// the files over the read bound a test sets in place of the tree's.
type staged struct {
	*proof.Tree

	blobs   map[string][]byte
	classes map[string]model.FileClass
	large   map[string]bool
}

func (s staged) Read(path string) ([]byte, error) {
	if s.large[path] {
		return nil, fmt.Errorf("%w: %s", model.ErrTooLarge, path)
	}
	data, err := s.Tree.Read(path)
	if err != nil {
		return nil, fmt.Errorf("staged: %w", err)
	}
	return data, nil
}

func (s staged) Blob(id string) ([]byte, error) {
	data, ok := s.blobs[id]
	if !ok {
		return nil, fmt.Errorf("%w: blob %s", model.ErrUnreadable, id)
	}
	return data, nil
}

func (s staged) Class(path string) model.FileClass {
	if class, ok := s.classes[path]; ok {
		return class
	}
	return s.Tree.Class(path)
}

// numbered is n lines, each naming its number after the prefix.
func numbered(prefix string, n int) string {
	out := make([]string, 0, n)
	for i := range n {
		out = append(out, fmt.Sprintf("%s %d", prefix, i))
	}
	return strings.Join(out, "\n") + "\n"
}

// shaped runs commit.shape over a chore commit of the changes on a
// snapshot.
func shaped(t *testing.T, snap model.Snapshot, changes ...model.Change) model.Result {
	t.Helper()
	in := commitInput(model.Commit{Message: "chore: tidy the tables\n"}, snap)
	in.Changes = changes
	res, err := commits.Shape().Run(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// wantShape requires no finding at all when want is empty, so a near case
// that yields two false findings fails too, and otherwise exactly one
// commit-shape finding whose text holds want.
func wantShape(t *testing.T, name string, res model.Result, want string) {
	t.Helper()
	if want == "" {
		if len(res.Findings) != 0 {
			t.Errorf("%s: findings %+v, want none", name, res.Findings)
		}
		return
	}
	if len(res.Findings) != 1 || res.Findings[0].Code != "commit-shape" || !strings.Contains(res.Findings[0].Message, want) {
		t.Errorf("%s: findings %+v, want one commit-shape finding with %q", name, res.Findings, want)
	}
}

func TestShapeCountsTheFilesOfACommitThatIsNotFeat(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		subject string
		files   int
		want    string
	}{
		"chore over the files":   {"chore: dump the tables", 41, "a chore commit adds 41 new files; split it"},
		"chore at the limit":     {"chore: dump the tables", 40, ""},
		"feat over the files":    {"feat: add the tables", 41, ""},
		"a breaking fix":         {"fix(api)!: drop the tables", 41, "a fix commit adds 41 new files"},
		"a subject without type": {"dump everything", 41, "a typeless commit adds 41 new files"},
		"a merge git wrote":      {"Merge branch 'topic' into main", 41, ""},
		"an autosquash of feat":  {"fixup! feat: add the tables", 41, ""},
		"an empty message":       {"", 41, ""},
	}
	for name, c := range cases {
		res, err := commits.Shape().Run(t.Context(), added(t, c.subject, c.files))
		if err != nil || res.Status.Kind != model.StatusMeasured {
			t.Fatalf("%s: %+v, %v", name, res, err)
		}
		wantShape(t, name, res, c.want)
	}
}

func TestShapeCountsTheNewLinesOfEveryChange(t *testing.T) {
	t.Parallel()
	old := numbered("kept", 300)
	files := map[string][]byte{
		"big.txt": []byte(numbered("new", 401)), "full.txt": []byte(numbered("new", 400)),
		"grown.txt": []byte(old + numbered("new", 401)), "moved.txt": []byte(numbered("new", 400) + old),
		"image.png": []byte("\x00" + strings.Repeat("\n", 500)), "renamed.txt": []byte(old), "empty.txt": nil,
	}
	for i := range 41 {
		files[fmt.Sprintf("m%02d.txt", i+1)] = []byte(old + "added\n")
	}
	files["copied.txt"], files["copied-near.txt"] = []byte(strings.Repeat("x\n", 402)), []byte(strings.Repeat("x\n", 401))
	snap := staged{Tree: proof.NewTree(files), blobs: map[string][]byte{"old": []byte(old), "x": []byte("x\n")}}
	add := func(path string) model.Change {
		return model.Change{Op: model.ChangeAdd, NewPath: path, Mode: "100644", After: "new"}
	}
	modify := func(path string) model.Change {
		return model.Change{Op: model.ChangeModify, OldPath: path, NewPath: path, Mode: "100644", Before: "old", After: "new"}
	}
	copied := func(path string) model.Change {
		return model.Change{Op: model.ChangeModify, OldPath: path, NewPath: path, Mode: "100644", Before: "x", After: "new"}
	}
	touched := make([]model.Change, 0, 41)
	for i := range 41 {
		touched = append(touched, modify(fmt.Sprintf("m%02d.txt", i+1)))
	}
	cases := map[string]struct {
		changes []model.Change
		lines   string
	}{
		"an added file over the lines":       {[]model.Change{add("big.txt")}, "stages 401 new lines"},
		"an added file at the limit":         {[]model.Change{add("full.txt")}, ""},
		"a modified file over the lines":     {[]model.Change{modify("grown.txt")}, "stages 401 new lines"},
		"41 modified files are no new files": {touched, ""},
		"moved lines are no new lines":       {[]model.Change{modify("moved.txt")}, ""},
		"a line copied 401 more times":       {[]model.Change{copied("copied.txt")}, "stages 401 new lines"},
		"a line copied 400 more times":       {[]model.Change{copied("copied-near.txt")}, ""},
		"a binary file has no lines":         {[]model.Change{add("image.png")}, ""},
		"an empty file has no lines":         {[]model.Change{add("empty.txt")}, ""},
		"a link and a submodule":             {[]model.Change{{Op: model.ChangeAdd, NewPath: "big.txt", Mode: "120000"}, {Op: model.ChangeAdd, NewPath: "full.txt", Mode: "160000"}}, ""},
		"a rename that keeps its blob":       {[]model.Change{{Op: model.ChangeRename, OldPath: "a.txt", NewPath: "renamed.txt", Mode: "100644", Before: "same", After: "same"}}, ""},
		"deletions are no new files":         {deletions(41), ""},
	}
	for name, c := range cases {
		res := shaped(t, snap, c.changes...)
		if res.Status.Kind != model.StatusMeasured {
			t.Errorf("%s: %+v, want it measured", name, res)
		}
		wantShape(t, name, res, c.lines)
	}
}

// deletions is n staged deletions.
func deletions(n int) []model.Change {
	out := make([]model.Change, 0, n)
	for i := range n {
		out = append(out, model.Change{Op: model.ChangeDelete, OldPath: fmt.Sprintf("gone%02d.txt", i), Mode: "100644", Before: "old"})
	}
	return out
}

func TestShapeLeavesUnmeasuredWhatItCannotRead(t *testing.T) {
	t.Parallel()
	tree := proof.NewTree(map[string][]byte{
		"a.txt": []byte("a\n"), "huge.txt": []byte("x\n"), "odd.txt": []byte("b\n"),
	})
	snap := staged{Tree: tree, classes: map[string]model.FileClass{"odd.txt": model.ClassUnreadable}, large: map[string]bool{"huge.txt": true}}
	lost := shaped(t, snap, model.Change{Op: model.ChangeModify, OldPath: "a.txt", NewPath: "a.txt", Mode: "100644", Before: "gone", After: "new"})
	if lost.Status.Kind != model.StatusUnmeasured || !strings.Contains(lost.Status.Reason, "a.txt") {
		t.Errorf("an old blob the snapshot lacks: %+v", lost)
	}
	odd := shaped(t, snap, model.Change{Op: model.ChangeAdd, NewPath: "odd.txt", Mode: "100644", After: "new"})
	missing := shaped(t, snap, model.Change{Op: model.ChangeAdd, NewPath: "absent.txt", Mode: "100644", After: "new"})
	if odd.Status.Kind != model.StatusUnmeasured || missing.Status.Kind != model.StatusUnmeasured {
		t.Errorf("an unreadable blob = %+v, a path the snapshot lacks = %+v", odd, missing)
	}
	huge := shaped(t, snap, model.Change{Op: model.ChangeAdd, NewPath: "huge.txt", Mode: "100644", After: "new"})
	if huge.Status.Kind != model.StatusMeasured || !slices.Equal(huge.Unread, []string{"huge.txt"}) || len(huge.Findings) != 0 {
		t.Errorf("a file over the read bound = %+v", huge)
	}
}

// A commit over the file limit is refused on its files alone, before any
// blob is read, so a blob it cannot read never leaves it unmeasured.
func TestShapeCountsTheFilesBeforeReadingABlob(t *testing.T) {
	t.Parallel()
	files := map[string][]byte{}
	changes := make([]model.Change, 0, 41)
	for i := range 41 {
		path := fmt.Sprintf("f%02d.txt", i+1)
		files[path] = []byte("line\n")
		changes = append(changes, model.Change{Op: model.ChangeAdd, NewPath: path, Mode: "100644", After: "new"})
	}
	snap := staged{Tree: proof.NewTree(files), classes: map[string]model.FileClass{"f01.txt": model.ClassUnreadable}}
	res := shaped(t, snap, changes...)
	if res.Status.Kind != model.StatusMeasured {
		t.Fatalf("41 new files, one of them unreadable: %+v, want it measured", res)
	}
	wantShape(t, "41 new files, one of them unreadable", res, "a chore commit adds 41 new files")
}

func TestShapeAgreesWithTheShellFamily(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		subject string
		files   int
		want    bool
	}{
		{"chore: dump", 41, true}, {"chore: dump", 40, false}, {"feat: dump", 41, false}, {"dump", 41, true}, {"refactor(x): dump", 41, true},
	} {
		names := make([]string, 0, c.files)
		for i := range c.files {
			names = append(names, fmt.Sprintf("f%02d.txt", i+1))
		}
		command := "git add " + strings.Join(names, " ") + " && " + message{c.subject, ""}.command()
		shellGot := shellCommandFinds(t, shell.Shape(), "commit-shape", command)
		res, err := commits.Shape().Run(t.Context(), added(t, c.subject, c.files))
		if err != nil || shellGot != c.want || hasCode(res, "commit-shape") != c.want {
			t.Errorf("%q with %d files: shell %v, commit %+v, want %v", c.subject, c.files, shellGot, res, c.want)
		}
	}
}
