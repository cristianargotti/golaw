package codex_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestPatchRoutes(t *testing.T) {
	t.Parallel()
	for _, route := range []string{
		"apply-patch-add", "apply-patch-update", "apply-patch-unapplied",
		"apply-patch-move", "apply-patch-delete",
	} {
		for _, tt := range []struct {
			side string
			want hook.Outcome
		}{
			{"red", hook.Deny},
			{"near", hook.Allow},
		} {
			t.Run(route+"/"+tt.side, func(t *testing.T) {
				t.Parallel()
				_, result := routeResult(t, route+"/"+tt.side)
				if result.Outcome != tt.want {
					t.Fatalf("want %s, got %s: %s",
						tt.want, result.Outcome, result.Reason)
				}
				if route == "apply-patch-unapplied" &&
					tt.side == "near" && result.Notice == "" {
					t.Fatal("unapplied candidate needs a notice")
				}
			})
		}
	}
}

// candidateCase is one row of TestPatchCandidates.
type candidateCase struct {
	name  string
	body  string
	files map[string][]byte
	op    model.CandidateOp
	path  string
	from  string
	bytes string
}

func TestPatchCandidates(t *testing.T) {
	t.Parallel()
	tests := []candidateCase{
		{
			"create", "*** Add File: notes.md\n+new\n", nil,
			model.CandidateCreate, "notes.md", "", "new\n",
		},
		{
			"replace", "*** Add File: notes.md\n+new\n",
			map[string][]byte{"notes.md": []byte("old\n")},
			model.CandidateReplace, "notes.md", "", "new\n",
		},
		{
			"edit", "*** Update File: notes.md\n@@\n-old\n+new\n",
			map[string][]byte{"notes.md": []byte("before\nold\nafter\n")},
			model.CandidateEdit, "notes.md", "", "before\nnew\nafter\n",
		},
		{
			"move", "*** Update File: notes.md\n*** Move to: moved.md\n@@\n-old\n+new\n",
			map[string][]byte{"notes.md": []byte("old\n")},
			model.CandidateMove, "moved.md", "notes.md", "new\n",
		},
		{
			"delete", "*** Delete File: notes.md\n",
			map[string][]byte{"notes.md": []byte("old\n")},
			model.CandidateDelete, "notes.md", "", "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := commandPayload(t, "pre-apply-patch-add.json", patch(tt.body))
			assertCandidate(t, tt, decodeRequest(t, data, tt.files))
		})
	}
}

// assertCandidate holds the one candidate of a row to its operation, its
// paths, its bytes, its reconstruction and its base hash.
func assertCandidate(t *testing.T, tt candidateCase, request hook.Request) {
	t.Helper()
	if len(request.Event.Candidates) != 1 {
		t.Fatalf("candidates: %+v", request.Event.Candidates)
	}
	candidate := request.Event.Candidates[0]
	if candidate.Op != tt.op || candidate.Path != tt.path ||
		candidate.From != tt.from {
		t.Fatalf("candidate: %+v", candidate)
	}
	if string(request.Content[tt.path]) != tt.bytes {
		t.Fatalf("content = %q", request.Content[tt.path])
	}
	if tt.op != model.CandidateDelete && !candidate.Reconstructed {
		t.Fatal("whole candidate was not reconstructed")
	}
	basePath := tt.path
	if tt.from != "" {
		basePath = tt.from
	}
	if base, exists := tt.files[basePath]; exists &&
		candidate.BaseHash != hook.Hash(base) {
		t.Fatal("base hash was lost")
	}
}

func TestPartialUsesOnlyAddedLines(t *testing.T) {
	t.Parallel()
	dash := string(rune(0x2014))
	text := patch("*** Update File: notes.md\n@@\n context " +
		dash + "\n-missing\n+safe\n")
	data := commandPayload(t, "pre-apply-patch-add.json", text)
	files := map[string][]byte{"notes.md": []byte("different\n")}
	request := decodeRequest(t, data, files)
	if request.Event.Candidates[0].Reconstructed ||
		string(request.Content["notes.md"]) != "safe\n" ||
		len(request.Gaps) != 0 {
		t.Fatalf("incorrect partial candidate: %+v", request)
	}
	_, result := hooktest.Evaluate(t, "codex", adapter(), data, files)
	if result.Outcome != hook.Allow ||
		!strings.Contains(result.Notice, "only its added lines were judged") {
		t.Fatalf("incorrect partial judgment: %+v", result)
	}
}

func TestEmptyPatchIsNoOp(t *testing.T) {
	t.Parallel()
	data := commandPayload(t, "pre-apply-patch-add.json", patch(""))
	request := decodeRequest(t, data, nil)
	if !request.Observe || len(request.Event.Candidates) != 0 {
		t.Fatalf("incorrect empty patch: %+v", request)
	}
	_, result := hooktest.Evaluate(t, "codex", adapter(), data, nil)
	if result.Outcome != hook.Allow {
		t.Fatalf("empty patch refused: %+v", result)
	}
}

// recordedCase is one row of TestRecordedPatchRequests.
type recordedCase struct {
	name       string
	target     string
	files      map[string][]byte
	candidates []model.Candidate
	content    map[string]string
}

func TestRecordedPatchRequests(t *testing.T) {
	t.Parallel()
	old := []byte("package app\n\nfunc Old() {\n\treturn\n}\n")
	gone := []byte("package app\n")
	for _, tt := range []recordedCase{
		{
			"pre-apply-patch-add.json", "internal/app/hello.go", nil,
			[]model.Candidate{{Path: "internal/app/hello.go", Op: model.CandidateCreate}},
			map[string]string{"internal/app/hello.go": "package app\n\nfunc Hello() string { return \"hi\" }\n"},
		},
		{
			"pre-apply-patch-update-move-delete.json", "internal/app/old.go",
			map[string][]byte{"internal/app/old.go": old, "internal/app/gone.go": gone},
			[]model.Candidate{
				{
					Path: "internal/app/new.go", Op: model.CandidateMove,
					From: "internal/app/old.go", BaseHash: hook.Hash(old),
				},
				{
					Path: "internal/app/gone.go", Op: model.CandidateDelete,
					BaseHash: hook.Hash(gone),
				},
			},
			map[string]string{"internal/app/new.go": "package app\n\nfunc Old() {\n\treturn // moved\n}\n"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertRecorded(t, tt, decodeRequest(t, corpus(t, tt.name), tt.files))
		})
	}
}

// assertRecorded holds a decoded corpus request to its event, to each of
// its candidates and to the content of each path.
func assertRecorded(t *testing.T, tt recordedCase, request hook.Request) {
	t.Helper()
	if request.Event.Shape != model.ShapeWrite ||
		request.Event.Tool != "apply_patch" || request.Event.Target != tt.target {
		t.Fatalf("unexpected event: %+v", request.Event)
	}
	if len(request.Event.Candidates) != len(tt.candidates) {
		t.Fatalf("candidates: %+v", request.Event.Candidates)
	}
	for i, want := range tt.candidates {
		got := request.Event.Candidates[i]
		if got.Path != want.Path || got.Op != want.Op ||
			got.From != want.From || got.BaseHash != want.BaseHash {
			t.Fatalf("candidate %d: %+v, want %+v", i, got, want)
		}
		if got.Op != model.CandidateDelete && !got.Reconstructed {
			t.Fatalf("candidate %d was not reconstructed", i)
		}
	}
	for path, want := range tt.content {
		if string(request.Content[path]) != want {
			t.Fatalf("content of %s = %q", path, request.Content[path])
		}
	}
}
