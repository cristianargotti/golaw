package codex_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/codex"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// A here-document patch is the same write as the patch tool's own event,
// so a base over the read bound gets the answer the tool gets: the file
// judged by its name with the notice of what was not measured alone, and
// cannot judge with the fix named beside bytes the hook holds.
func TestHeredocPatchOverTheReadBoundAnswersAsThePatchTool(t *testing.T) {
	t.Parallel()
	dash := string(rune(0x2014))
	files := map[string][]byte{"big.txt": []byte(strings.Repeat("x\n", model.MaxReadBytes/2+1))}
	for _, tt := range []struct {
		name, body string
		want       hook.Outcome
		cannot     bool
	}{
		{"near update alone", "*** Update File: big.txt\n@@\n-x\n+y\n", hook.Allow, false},
		{"red update beside an added file", "*** Update File: big.txt\n@@\n-x\n+y\n*** Add File: added.md\n+hello " + dash + " world\n", hook.Deny, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			shell := commandPayload(t, "pre-bash-plain.json", "apply_patch <<'EOF'\n"+patch(tt.body)+"EOF")
			_, ours := hooktest.Evaluate(t, "codex", adapter(), shell, files)
			_, tool := hooktest.Evaluate(t, "codex", adapter(), commandPayload(t, "pre-apply-patch-add.json", patch(tt.body)), files)
			if ours.Outcome != tt.want || ours.CannotJudge != tt.cannot {
				t.Fatalf("want %s, cannot judge %v, got %+v", tt.want, tt.cannot, ours)
			}
			if ours.Outcome != tool.Outcome || ours.Reason != tool.Reason || ours.Notice != tool.Notice {
				t.Fatalf("here-document %+v, patch tool %+v", ours, tool)
			}
			if tt.want == hook.Allow && !strings.Contains(ours.Notice, "big.txt") {
				t.Fatalf("the file over the bound is not named: %q", ours.Notice)
			}
		})
	}
}

// An empty patch writes nothing, so the command is judged alone; a patch
// whose grammar is broken cannot be judged, and its parse names no file.
func TestEmptyAndMalformedHeredocPatches(t *testing.T) {
	t.Parallel()
	empty := commandPayload(t, "pre-bash-plain.json", "apply_patch <<'EOF'\n*** Begin Patch\n*** End Patch\nEOF")
	if request := decodeRequest(t, empty, nil); request.Writes != nil || request.Event.Shape != model.ShapeShell {
		t.Fatalf("empty patch became a write: %+v", request)
	}
	if _, result := hooktest.Evaluate(t, "codex", adapter(), empty, nil); result.Outcome != hook.Allow {
		t.Fatalf("empty patch refused: %+v", result)
	}
	broken := commandPayload(t, "pre-bash-plain.json", "apply_patch <<'EOF'\n*** Begin Patch\n*** Rename File: notes.md\n*** End Patch\nEOF")
	if parsed, err := codex.Parse(broken); err != nil || parsed.Event.Target != "" {
		t.Fatalf("broken patch target = %q, %v", parsed.Event.Target, err)
	}
	if _, result := hooktest.Evaluate(t, "codex", adapter(), broken, nil); !result.CannotJudge || result.Outcome == hook.Allow {
		t.Fatalf("broken patch was allowed: %+v", result)
	}
}

// Literal forms the shell writes as they read are recognized, however
// their words are spelled; a word the shell would read otherwise than its
// literal value, an operator outside the forms or a here-document that
// never ends stays the shell family's.
func TestHeredocWordsAndOperators(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, command, path, bytes string
	}{
		{"adjacent redirect", "cat >notes.md <<EOF\nhello\nEOF", "notes.md", "hello\n"},
		{"quoted target", "cat > \"notes.md\" <<'EOF'\nhello\nEOF", "notes.md", "hello\n"},
		{"partly quoted delimiter", "cat > notes.md <<E\"O\"F\nhello $x\nEOF", "notes.md", "hello $x\n"},
		{"append last", "cat <<'EOF' >> notes.md\nhello\nEOF\n\n", "notes.md", "hello\n"},
		{"unicode target", "cat > nótas.md <<'EOF'\nhello\nEOF", "nótas.md", "hello\n"},
	} {
		request := decodeRequest(t, commandPayload(t, "pre-bash-plain.json", tt.command), nil)
		if request.Writes == nil || len(request.Writes.Candidates) != 1 ||
			request.Writes.Candidates[0].Path != tt.path || string(request.Writes.Content[tt.path]) != tt.bytes {
			t.Fatalf("%s: unexpected writes %+v", tt.name, request.Writes)
		}
	}
	for _, command := range []string{
		"cat > ' notes.md' <<'EOF'\nhello\nEOF",
		"cat > '~/notes.md' <<'EOF'\nhello\nEOF",
		"cat > '' <<'EOF'\nhello\nEOF",
		"cat >| notes.md <<'EOF'\nhello\nEOF",
		"cat >& notes.md <<'EOF'\nhello\nEOF",
		"cat > notes.md 2> err.txt <<'EOF'\nhello\nEOF",
		"cat > notes*.md <<'EOF'\nhello\nEOF",
		"cat > 'notes.md <<'EOF'\nhello\nEOF",
		"cat > notes.md <<\"EOF\nhello\nEOF",
		"cat > notes.md <<''\nhello\n\n",
		"cat > notes.md <<'EOF'\nhello",
		"cat > notes.md <<'EOF'",
		"'cat' > notes.md <<'EOF'\nhello\nEOF",
		"cat > notes.md < in.txt <<'EOF'\nhello\nEOF",
		"cat > notes.md <<'EOF'; true\nhello\nEOF",
		"# cat > notes.md <<'EOF'\nhello\nEOF",
		"cd sub && 'apply_patch' <<'EOF'\n" + patch("*** Add File: notes.md\n+hello\n") + "EOF",
		"cd sub dir && apply_patch <<'EOF'\n" + patch("*** Add File: notes.md\n+hello\n") + "EOF",
	} {
		request := decodeRequest(t, commandPayload(t, "pre-bash-plain.json", command), nil)
		if request.Writes != nil || request.Event.Shape != model.ShapeShell {
			t.Fatalf("unrecognized command became a write: %q", command)
		}
	}
}

// A replacement binds the base it replaces, a base over the read bound
// binds none, and a base that cannot be read cannot be judged.
func TestCatReplacementBase(t *testing.T) {
	t.Parallel()
	data := commandPayload(t, "pre-bash-plain.json", "cat > notes.md <<'EOF'\nnew\nEOF")
	for _, tt := range []struct {
		name string
		base []byte
		hash string
	}{
		{"readable", []byte("old\n"), hook.Hash([]byte("old\n"))},
		{"over the read bound", []byte(strings.Repeat("x\n", model.MaxReadBytes/2+1)), ""},
	} {
		request := decodeRequest(t, data, map[string][]byte{"notes.md": tt.base})
		want := model.Candidate{Path: "notes.md", Op: model.CandidateReplace, Reconstructed: true, BaseHash: tt.hash}
		if request.Writes == nil || len(request.Writes.Candidates) != 1 || request.Writes.Candidates[0] != want {
			t.Fatalf("%s: candidates %+v", tt.name, request.Writes)
		}
	}
	root := hooktest.Repository(t)
	parsed, err := codex.Parse(hooktest.Payload(data, root))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := unreadableSnapshot{Snapshot: proof.NewTree(map[string][]byte{"notes.md": []byte("old\n")})}
	if _, err = parsed.Decode(hook.Base{Root: root}, snapshot); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatalf("want cannot judge, got %v", err)
	}
}
