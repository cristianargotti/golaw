package hook_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestIdentitiesDetectContentChanges(t *testing.T) {
	t.Parallel()
	before, err := hook.Identities(t.Context(), proof.NewTree(map[string][]byte{
		"dirty.go": []byte("old"), "gone.go": {}, "same.go": []byte("same"),
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	after, err := hook.Identities(t.Context(), proof.NewTree(map[string][]byte{
		"dirty.go": []byte("new"), "new.go": {}, "same.go": []byte("same"),
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(hook.Changed(before, after),
		[]string{"dirty.go", "gone.go", "new.go"}) {
		t.Fatal(hook.Changed(before, after))
	}
	absent, err := hook.Identities(t.Context(), proof.NewTree(nil), []string{"empty.go"})
	if err != nil {
		t.Fatal(err)
	}
	empty, err := hook.Identities(t.Context(),
		proof.NewTree(map[string][]byte{"empty.go": {}}), []string{"empty.go"})
	if err != nil {
		t.Fatal(err)
	}
	if maps.Equal(absent, empty) {
		t.Fatal("absence equals empty content")
	}
	if hook.MapHash(before) != hook.MapHash(maps.Clone(before)) {
		t.Fatal("identity depends on map iteration")
	}
}

func TestStateKeyIsolation(t *testing.T) {
	t.Parallel()
	base := model.Event{
		Agent: "claude", SessionID: "session",
		AgentID: "agent", ToolUseID: "tool",
	}
	original, err := hook.StateKey("repository", base)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 5 {
		event, repository := base, "repository"
		switch i {
		case 0:
			repository = "other"
		case 1:
			event.SessionID = "other"
		case 2:
			event.AgentID = "other"
		case 3:
			event.ToolUseID = "other"
		case 4:
			event.Agent = "codex"
		}
		key, err := hook.StateKey(repository, event)
		if err != nil || key == original {
			t.Fatalf("identity collision: %d, %v", i, err)
		}
	}
	base.ToolUseID = ""
	if _, err := hook.StateKey("repository", base); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatal(err)
	}
}

func TestStateKeyRefusesAnIncompleteIdentity(t *testing.T) {
	t.Parallel()
	whole := model.Event{Agent: "claude", SessionID: "session", ToolUseID: "tool"}
	if _, err := hook.StateKey("repository", whole); err != nil {
		t.Fatal("an event without agent_id was refused:", err)
	}
	for _, tc := range []struct {
		repository string
		event      model.Event
	}{
		{"", whole},
		{"repository", model.Event{SessionID: "session", ToolUseID: "tool"}},
		{"repository", model.Event{Agent: "claude", ToolUseID: "tool"}},
		{"repository", model.Event{Agent: "claude", SessionID: "session"}},
	} {
		if _, err := hook.StateKey(tc.repository, tc.event); !errors.Is(err, model.ErrCannotJudge) {
			t.Fatal(tc, err)
		}
	}
}

// sizedSnapshot answers every file from one shared buffer, so the total
// byte bound is reached without holding every file in memory.
type sizedSnapshot struct {
	count int
	body  []byte
}

func (s sizedSnapshot) Files() []model.File {
	files := make([]model.File, s.count)
	for i := range files {
		files[i] = model.File{Path: fmt.Sprintf("file-%d.go", i), Class: model.ClassTracked}
	}
	return files
}

func (sizedSnapshot) Class(string) model.FileClass { return model.ClassTracked }

func (s sizedSnapshot) Read(string) ([]byte, error) { return s.body, nil }

func TestIdentityByteBoundAndOversizedRead(t *testing.T) {
	t.Parallel()
	body := make([]byte, model.MaxReadBytes)
	if _, err := hook.Identities(t.Context(), sizedSnapshot{count: 8, body: body}, nil); err != nil {
		t.Fatal("64 MiB of content was refused:", err)
	}
	if _, err := hook.Identities(t.Context(), sizedSnapshot{count: 9, body: body}, nil); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatal(err)
	}
	oversized := sizedSnapshot{count: 1, body: make([]byte, model.MaxReadBytes+1)}
	if _, err := hook.Identities(t.Context(), oversized, nil); !errors.Is(err, model.ErrTooLarge) {
		t.Fatal(err)
	}
}

func TestIdentityBoundsAndCancellation(t *testing.T) {
	t.Parallel()
	tree := proof.NewTree(map[string][]byte{
		"large": make([]byte, model.MaxReadBytes+1),
	})
	if _, err := hook.Identities(t.Context(), tree, nil); err == nil {
		t.Fatal("oversized content accepted")
	}
	if _, err := hook.Identities(t.Context(), proof.NewTree(nil),
		[]string{"../outside"}); err == nil {
		t.Fatal("escaping path accepted")
	}
}

func TestIdentityPathLimitAndCancelledCapture(t *testing.T) {
	t.Parallel()
	paths := make([]string, 65536)
	for i := range paths {
		paths[i] = fmt.Sprintf("file-%d.go", i)
	}
	if got, err := hook.Identities(t.Context(), proof.NewTree(nil), paths); err != nil || len(got) != len(paths) {
		t.Fatalf("valid repository capture failed: %d, %v", len(got), err)
	}
	over := make([]string, 0, len(paths)+1)
	over = append(over, paths...)
	over = append(over, "overflow.go")
	if _, err := hook.Identities(t.Context(), proof.NewTree(nil), over); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := hook.Identities(ctx, proof.NewTree(nil), []string{"a.go"}); !errors.Is(err, model.ErrDeadline) {
		t.Fatal(err)
	}
}

// linkedSnapshot answers like the worktree for the classes that hold no
// bytes: listed with the size the listing knows, refused on read.
type linkedSnapshot struct {
	files []model.File
}

func (s linkedSnapshot) Files() []model.File { return s.files }

func (s linkedSnapshot) Class(path string) model.FileClass {
	for _, file := range s.files {
		if file.Path == path {
			return file.Class
		}
	}
	return model.ClassUnknown
}

func (linkedSnapshot) Read(path string) ([]byte, error) {
	return nil, fmt.Errorf("%w: %s holds no bytes", model.ErrUnreadable, path)
}

func TestLinksAndSubmodulesHaveAnIdentityWithoutARead(t *testing.T) {
	t.Parallel()
	link := model.File{Path: "link.go", Class: model.ClassSymlink, Size: 4}
	module := model.File{Path: "vendor/mod", Class: model.ClassGitlink}
	before, err := hook.Identities(t.Context(), linkedSnapshot{files: []model.File{link, module}}, nil)
	if err != nil {
		t.Fatal("a link or a submodule stopped the capture:", err)
	}
	retarget := link
	retarget.Size = 9
	after, err := hook.Identities(t.Context(), linkedSnapshot{files: []model.File{retarget, module}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(hook.Changed(before, after), []string{"link.go"}) {
		t.Fatal(hook.Changed(before, after))
	}
	listed, err := hook.Identities(t.Context(), linkedSnapshot{files: []model.File{module}},
		[]string{"vendor/mod", "gone"})
	if err != nil || listed["vendor/mod"] == listed["gone"] {
		t.Fatalf("a listed submodule reads as absent: %v, %v", listed, err)
	}
	unreadable := model.File{Path: "locked.go", Class: model.ClassUnreadable}
	if _, err := hook.Identities(t.Context(), linkedSnapshot{files: []model.File{unreadable}}, nil); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatal("an unreadable file was identified without its bytes:", err)
	}
}
