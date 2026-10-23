package codex_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/codex"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
)

// The core judges a file over the read bound by its name alone, so a patch
// that changes one beside files whose bytes the hook holds is cannot judge
// with the fix named, never allowed with those bytes unjudged; a patch
// that changes it alone, or beside a deletion, keeps the notice of the
// file it did not measure, as the Claude adapter answers an edit of one.
func TestPatchBesideABaseOverTheReadBound(t *testing.T) {
	t.Parallel()
	dash := string(rune(0x2014))
	files := map[string][]byte{
		"big.txt":  []byte(strings.Repeat("x\n", model.MaxReadBytes/2+1)),
		"notes.md": []byte("old\n"),
	}
	for _, tt := range []struct {
		name, body string
		want       hook.Outcome
		cannot     bool
	}{
		{"red update beside an added file", "*** Update File: big.txt\n@@\n-x\n+y\n*** Add File: added.md\n+hello " + dash + " world\n", hook.Deny, true},
		{"red move beside an edit", "*** Update File: notes.md\n@@\n-old\n+new\n*** Update File: big.txt\n*** Move to: moved.txt\n@@\n-x\n+y\n", hook.Deny, true},
		{"near update alone", "*** Update File: big.txt\n@@\n-x\n+y\n", hook.Allow, false},
		{"near update beside a deletion", "*** Update File: big.txt\n@@\n-x\n+y\n*** Delete File: notes.md\n", hook.Allow, false},
		{"near replace of the large file", "*** Add File: big.txt\n+hello " + dash + " world\n", hook.Deny, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := commandPayload(t, "pre-apply-patch-add.json", patch(tt.body))
			_, result := hooktest.Evaluate(t, "codex", adapter(), data, files)
			if result.Outcome != tt.want || result.CannotJudge != tt.cannot {
				t.Fatalf("want %s, cannot judge %v, got %+v", tt.want, tt.cannot, result)
			}
			if tt.cannot && (!strings.Contains(result.Reason, "big.txt") ||
				!strings.Contains(result.Reason, "apply its change in a separate patch")) {
				t.Fatalf("the fix is not named: %q", result.Reason)
			}
			if tt.want == hook.Allow && !strings.Contains(result.Notice, "big.txt") {
				t.Fatalf("the file over the bound is not named: %q", result.Notice)
			}
		})
	}
}

// A deletion needs no bytes, so its candidate is whole and the allow of a
// deletion carries no notice of a change that could not be reconstructed.
func TestDeletionIsAWholeCandidate(t *testing.T) {
	t.Parallel()
	data := commandPayload(t, "pre-apply-patch-add.json", patch("*** Delete File: notes.md\n"))
	files := map[string][]byte{"notes.md": []byte("old\n")}
	request := decodeRequest(t, data, files)
	if len(request.Event.Candidates) != 1 || !request.Event.Candidates[0].Reconstructed {
		t.Fatalf("deletion candidate: %+v", request.Event.Candidates)
	}
	_, result := hooktest.Evaluate(t, "codex", adapter(), data, files)
	if result.Outcome != hook.Allow || result.Notice != "" {
		t.Fatalf("deletion answered %+v", result)
	}
}

// A patch without a hunk writes nothing in any environment, so the line
// that selects one changes nothing about its allow.
func TestEmptyPatchOfAnEnvironmentIsNoOp(t *testing.T) {
	t.Parallel()
	data := commandPayload(t, "pre-apply-patch-add.json", patch("*** Environment ID: remote\n"))
	_, result := hooktest.Evaluate(t, "codex", adapter(), data, nil)
	if result.Outcome != hook.Allow || result.CannotJudge {
		t.Fatalf("empty patch refused: %+v", result)
	}
}

// Codex joins a header path that starts with a tilde to its working
// directory, so the target enrollment reads is that file, never one under
// the home; the join writes the separator of the system.
func TestTildeHeaderIsJoinedToTheWorkingDirectory(t *testing.T) {
	t.Parallel()
	data := commandPayload(t, "pre-apply-patch-add.json", patch("*** Add File: ~/notes.md\n+hello\n"))
	parsed, err := codex.Parse(data)
	if err != nil || parsed.Event.Target != filepath.FromSlash("/project/~/notes.md") {
		t.Fatalf("target = %q, %v", parsed.Event.Target, err)
	}
	request := decodeRequest(t, data, nil)
	if len(request.Event.Candidates) != 1 || request.Event.Candidates[0].Path != "~/notes.md" {
		t.Fatalf("candidates: %+v", request.Event.Candidates)
	}
}
