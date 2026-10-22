package codex_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/codex"
	"github.com/cristianargotti/golaw/internal/model"
)

func patch(body string) string {
	return "*** Begin Patch\n" + body + "*** End Patch\n"
}

func TestPatchGrammar(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		text string
		bad  bool
	}{
		{"empty", patch(""), false},
		{"empty-add", patch("*** Add File: empty.txt\n"), false},
		{"add", patch("*** Add File: notes.txt\n+hello\n+\n"), false},
		{"delete", patch("*** Delete File: notes.txt\n"), false},
		{"implicit-update", patch("*** Update File: notes.txt\n-old\n+new\n"), false},
		{"context-only", patch("*** Update File: notes.txt\n@@\n old\n"), false},
		{"insertion", patch("*** Update File: notes.txt\n@@\n+new\n"), false},
		{"removal", patch("*** Update File: notes.txt\n@@\n-old\n"), false},
		{"move", patch("*** Update File: old.txt\n*** Move to: new.txt\n@@\n-old\n+new\n"), false},
		{"eof", patch("*** Update File: notes.txt\n@@\n-old\n+new\n*** End of File\n"), false},
		{"two-chunks", patch("*** Update File: notes.txt\n@@ first\n-a\n+b\n@@ second\n-c\n+d\n"), false},
		{"metadata", patch("*** Environment ID: local\n*** Add File: x\n+x\n"), false},
		{"blank-context", patch("*** Update File: x\n@@\n\n+x\n"), false},
		{"missing-begin", "*** Add File: x\n+x\n*** End Patch", true},
		{"missing-end", "*** Begin Patch\n*** Add File: x\n+x", true},
		{"trailing-text", patch("") + "extra", true},
		{"empty-update", patch("*** Update File: x\n"), true},
		{"move-only", patch("*** Update File: x\n*** Move to: y\n"), true},
		{"empty-chunk", patch("*** Update File: x\n@@\n"), true},
		{"two-empty-chunks", patch("*** Update File: x\n@@\n@@\n+x\n"), true},
		{"raw-add-line", patch("*** Add File: x\ntext\n"), true},
		{"late-move", patch("*** Update File: x\n@@\n+x\n*** Move to: y\n"), true},
		{"duplicate-move", patch("*** Update File: x\n*** Move to: y\n*** Move to: z\n@@\n+x\n"), true},
		{"empty-metadata", patch("*** Environment ID: \n"), true},
		{"repeated-metadata", patch("*** Environment ID: a\n*** Environment ID: b\n"), true},
		{"after-eof", patch("*** Update File: x\n@@\n+x\n*** End of File\n+y\n"), true},
		{"unknown-header", patch("*** Rename File: x\n"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := codex.ParsePatch(tt.text)
			if tt.bad {
				if !errors.Is(err, model.ErrCannotJudge) {
					t.Fatalf("want cannot judge, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPatchLeniency(t *testing.T) {
	t.Parallel()
	text := patch("*** Add File: x\n+hello\n")
	for _, candidate := range []string{
		text,
		strings.ReplaceAll(text, "\n", "\r\n"),
		" \n  " + strings.TrimSpace(text) + "  \n",
		"<<EOF\n" + text + "EOF",
		"<<'EOF'\n" + text + "EOF",
		"<<\"EOF\"\n" + text + "EOF",
	} {
		hunks, err := codex.ParsePatch(candidate)
		if err != nil {
			t.Fatal(err)
		}
		want := []codex.Hunk{{
			Op: model.CandidateCreate, Path: "x", Lines: []string{"hello"},
		}}
		if !reflect.DeepEqual(hunks, want) {
			t.Fatalf("got %+v, want %+v", hunks, want)
		}
	}
}

func TestPatchShape(t *testing.T) {
	t.Parallel()
	text := patch("*** Update File: old.txt\n*** Move to: new.txt\n" +
		"@@ heading\n keep\n-old\n+new\n*** End of File\n")
	hunks, err := codex.ParsePatch(text)
	if err != nil {
		t.Fatal(err)
	}
	want := []codex.Hunk{{
		Op: model.CandidateMove, Path: "old.txt", To: "new.txt",
		Chunks: []codex.Chunk{{
			Context: "heading",
			Old:     []string{"keep", "old"},
			New:     []string{"keep", "new"},
			EOF:     true,
		}},
	}}
	if !reflect.DeepEqual(hunks, want) {
		t.Fatalf("got %+v, want %+v", hunks, want)
	}
	if codex.PatchGrammar != "0.163.0" {
		t.Fatal("patch grammar pin changed")
	}
}
