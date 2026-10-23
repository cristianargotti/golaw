package codex_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestRecognizedHeredocs(t *testing.T) {
	t.Parallel()
	body := patch("*** Add File: notes.md\n+hello\n")
	for _, tt := range []struct {
		name    string
		command string
		path    string
		bytes   string
		whole   bool
	}{
		{
			"patch", "apply_patch <<'PATCH'\n" + body + "PATCH",
			"notes.md", "hello\n", true,
		},
		{
			"alias", "applypatch <<PATCH\n" + body + "PATCH",
			"notes.md", "hello\n", true,
		},
		{
			"quoted-delimiter", "apply_patch <<\"DONE\"\n" + body + "DONE",
			"notes.md", "hello\n", true,
		},
		{
			"cd", "cd sub && apply_patch <<'EOF'\n" + body + "EOF",
			"sub/notes.md", "hello\n", true,
		},
		{
			"quoted-cd", "cd 'sub dir' && apply_patch <<'EOF'\n" + body + "EOF",
			"sub dir/notes.md", "hello\n", true,
		},
		{
			"cat-first-target", "cat > notes.md <<'EOF'\nhello\nEOF",
			"notes.md", "hello\n", true,
		},
		{
			"cat-last-target", "cat <<'EOF' > notes.md\nhello\nEOF",
			"notes.md", "hello\n", true,
		},
		{
			"cat-append", "cat >> notes.md <<'EOF'\nhello\nEOF",
			"notes.md", "before\nhello\n", true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := commandPayload(t, "pre-bash-plain.json", tt.command)
			files := map[string][]byte{}
			if tt.name == "cat-append" {
				files["notes.md"] = []byte("before\n")
			}
			request := decodeRequest(t, data, files)
			if request.Event.Shape != model.ShapeShell ||
				request.Event.Shell == nil || request.Writes == nil ||
				len(request.Writes.Candidates) != 1 {
				t.Fatalf("unexpected request: %+v", request)
			}
			candidate := request.Writes.Candidates[0]
			if request.Event.Shell.Command != tt.command ||
				candidate.Path != tt.path || candidate.Reconstructed != tt.whole ||
				string(request.Writes.Content[tt.path]) != tt.bytes {
				t.Fatalf("unexpected write: %+v", request.Writes)
			}
		})
	}
}

func TestHeredocLimitsRemainShellOnly(t *testing.T) {
	t.Parallel()
	body := patch("*** Add File: notes.md\n+hello\n")
	for _, command := range []string{
		"apply_patch <<'EOF'\n" + body + "EOF\nprintf done",
		"cd sub || apply_patch <<'EOF'\n" + body + "EOF",
		"printf x | apply_patch <<'EOF'\n" + body + "EOF",
		"cd \"$TARGET\" && apply_patch <<'EOF'\n" + body + "EOF",
		"cd \"$(pwd)\" && apply_patch <<'EOF'\n" + body + "EOF",
		"cat > \"$TARGET\" <<'EOF'\nhello\nEOF",
		"cat > notes.md <<EOF\n$VALUE\nEOF",
		"cat > notes.md <<EOF\n$(printf hello)\nEOF",
		"cat > notes.md <<EOF\nhello \\\nworld\nEOF",
		"cat > notes.md <<'EOF'\nhello\nEOF\nfalse",
	} {
		data := commandPayload(t, "pre-bash-plain.json", command)
		request := decodeRequest(t, data, nil)
		if request.Writes != nil || request.Event.Shape != model.ShapeShell {
			t.Fatalf("unrecognized command became a write: %q", command)
		}
	}
}

// corpusCase is one row of TestHeredocCorpus.
type corpusCase struct {
	name    string
	command string
	files   map[string][]byte
	want    model.Candidate
	content string
}

func TestHeredocCorpus(t *testing.T) {
	t.Parallel()
	tests := []corpusCase{
		{
			name: "pre-bash-heredoc-patch.json",
			command: "apply_patch <<'PATCH'\n*** Begin Patch\n" +
				"*** Add File: notes/hello.txt\n+hello\n*** End Patch\nPATCH",
			want: model.Candidate{
				Path: "notes/hello.txt", Op: model.CandidateCreate,
				Reconstructed: true,
			},
			content: "hello\n",
		},
		{
			name: "pre-bash-heredoc-patch-cd.json",
			command: "cd sub/dir && apply_patch <<'EOF'\n*** Begin Patch\n" +
				"*** Update File: main.go\n@@\n-old\n+new\n*** End Patch\nEOF",
			files: map[string][]byte{"sub/dir/main.go": []byte("old\n")},
			want: model.Candidate{
				Path: "sub/dir/main.go", Op: model.CandidateEdit,
				Reconstructed: true, BaseHash: hook.Hash([]byte("old\n")),
			},
			content: "new\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			request := decodeRequest(t, corpus(t, tt.name), tt.files)
			assertCorpus(t, tt, request)
		})
	}
}

// assertCorpus holds a recorded here-document to its shell event, its
// command kept and its one write candidate with its bytes.
func assertCorpus(t *testing.T, tt corpusCase, request hook.Request) {
	t.Helper()
	if request.Event.Shape != model.ShapeShell ||
		request.Event.Tool != "Bash" || request.Event.Shell == nil {
		t.Fatalf("incorrect shell event: %+v", request.Event)
	}
	if request.Event.Shell.Tool != "Bash" ||
		request.Event.Shell.Command != tt.command {
		t.Fatalf("shell command changed: %+v", request.Event.Shell)
	}
	if request.Writes == nil ||
		!reflect.DeepEqual(request.Writes.Candidates, []model.Candidate{tt.want}) {
		t.Fatalf("incorrect shell candidates: %+v", request.Writes)
	}
	if string(request.Writes.Content[tt.want.Path]) != tt.content {
		t.Fatalf("incorrect shell content: %q", request.Writes.Content)
	}
}

// appendCase is one row of TestAppendHeredoc.
type appendCase struct {
	name    string
	path    string
	content string
	files   map[string][]byte
	want    hook.Outcome
	op      model.CandidateOp
	whole   bool
	notice  bool
}

func TestAppendHeredoc(t *testing.T) {
	t.Parallel()
	dash := string(rune(0x2014))
	tests := []appendCase{
		{
			"absent-hyphen", "new.md", "hello - world", nil,
			hook.Allow, model.CandidateCreate, true, false,
		},
		{
			"absent-dash", "new.md", "hello " + dash + " world", nil,
			hook.Deny, model.CandidateCreate, true, false,
		},
		{
			"present-hyphen", "notes.md", "hello - world",
			map[string][]byte{"notes.md": []byte("before\n")},
			hook.Allow, model.CandidateEdit, true, false,
		},
		{
			"present-dash", "notes.md", "hello " + dash + " world",
			map[string][]byte{"notes.md": []byte("before\n")},
			hook.Deny, model.CandidateEdit, true, false,
		},
		{
			"present-unterminated", "notes.md", "hello - world",
			map[string][]byte{"notes.md": []byte("before")},
			hook.Allow, model.CandidateEdit, true, false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			command := "cat >> " + tt.path + " <<'EOF'\n" + tt.content + "\nEOF"
			data := commandPayload(t, "pre-bash-plain.json", command)
			request := decodeRequest(t, data, tt.files)
			_, result := hooktest.Evaluate(t, "codex", adapter(), data, tt.files)
			assertAppend(t, tt, request, result)
		})
	}
}

// assertAppend holds an append to its one candidate, its appended bytes
// and the judgment of the hook with its notice.
func assertAppend(t *testing.T, tt appendCase, request hook.Request, result hook.Result) {
	t.Helper()
	if request.Writes == nil || len(request.Writes.Candidates) != 1 {
		t.Fatalf("append has no candidate: %+v", request)
	}
	candidate := request.Writes.Candidates[0]
	// A readable file the shell appends to becomes its bytes followed by
	// the body, exactly what the shell leaves, as an edit of Claude is.
	if candidate.Op != tt.op || candidate.Reconstructed != tt.whole ||
		string(request.Writes.Content[tt.path]) != string(tt.files[tt.path])+tt.content+"\n" {
		t.Fatalf("incorrect append candidate: %+v", request.Writes)
	}
	if result.Outcome != tt.want || result.CannotJudge {
		t.Fatalf("incorrect append judgment: %+v", result)
	}
	noticed := strings.Contains(result.Notice, "only its added lines were judged")
	if noticed != tt.notice {
		t.Fatalf("incorrect append notice: %+v", result)
	}
}

func TestHeredocPatchBase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		body    string
		present bool
	}{
		{"update-absent", "*** Update File: notes.md\n@@\n-old\n+new\n", false},
		{"update-present", "*** Update File: notes.md\n@@\n-old\n+new\n", true},
		{"move-absent", "*** Update File: notes.md\n*** Move to: moved.md\n@@\n-old\n+new\n", false},
		{"move-present", "*** Update File: notes.md\n*** Move to: moved.md\n@@\n-old\n+new\n", true},
		{"delete-absent", "*** Delete File: notes.md\n", false},
		{"delete-present", "*** Delete File: notes.md\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			files := map[string][]byte{}
			if tt.present {
				files["notes.md"] = []byte("old\n")
			}
			command := "apply_patch <<'PATCH'\n" + patch(tt.body) + "PATCH"
			data := commandPayload(t, "pre-bash-heredoc-patch.json", command)
			_, result := hooktest.Evaluate(t, "codex", adapter(), data, files)
			assertPatchBase(t, tt.present, result)
		})
	}
}

// assertPatchBase holds a here-document patch to a present base judged
// and to an absent one explained by its reason.
func assertPatchBase(t *testing.T, present bool, result hook.Result) {
	t.Helper()
	if present {
		if result.Outcome != hook.Allow || result.CannotJudge {
			t.Fatalf("present patch base refused: %+v", result)
		}
		return
	}
	if !result.CannotJudge || result.Outcome == hook.Allow ||
		!strings.Contains(result.Reason, "notes.md") ||
		!strings.Contains(result.Reason, "does not exist") {
		t.Fatalf("absent patch base not explained: %+v", result)
	}
}
