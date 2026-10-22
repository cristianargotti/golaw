package codex_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/codex"
	"github.com/cristianargotti/golaw/internal/model"
)

// The pinned parser compares the markers of an Update File hunk with only
// their trailing white space trimmed and reads every other line as a
// change line, so a patch it accepts must decode to the hunks it applies:
// a patch read otherwise would be judged on content Codex never writes, or
// refused although Codex applies it.
func TestPatchReadsUpdateLinesAsThePinnedParser(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		text string
		want []codex.Hunk
	}{
		{
			"blank line before the first chunk", patch("*** Update File: x\n\n+a\n"),
			[]codex.Hunk{updateX(codex.Chunk{Old: []string{""}, New: []string{"", "a"}})},
		},
		{
			"indented header inside an update", patch("*** Update File: x\n@@\n-a\n+b\n *** Add File: y\n+c\n"),
			[]codex.Hunk{updateX(codex.Chunk{
				Old: []string{"a", "*** Add File: y"},
				New: []string{"b", "*** Add File: y", "c"},
			})},
		},
		{
			"indented end anchor", patch("*** Update File: x\n@@\n-a\n+b\n *** End of File\n"),
			[]codex.Hunk{updateX(codex.Chunk{
				Old: []string{"a", "*** End of File"},
				New: []string{"b", "*** End of File"},
			})},
		},
		{
			"indented move", patch("*** Update File: x\n *** Move to: y\n-a\n+b\n"),
			[]codex.Hunk{updateX(codex.Chunk{
				Old: []string{"*** Move to: y", "a"},
				New: []string{"*** Move to: y", "b"},
			})},
		},
		{
			"indented chunk marker", patch("*** Update File: x\n @@\n-a\n+b\n"),
			[]codex.Hunk{updateX(codex.Chunk{Old: []string{"@@", "a"}, New: []string{"@@", "b"}})},
		},
		{
			"chunk context with trailing white space", patch("*** Update File: x\n@@ ctx  \n-a\n+b\n"),
			[]codex.Hunk{updateX(codex.Chunk{Context: "ctx", Old: []string{"a"}, New: []string{"b"}})},
		},
		{
			"end anchor before any chunk", patch("*** Update File: x\n*** End of File\n-a\n+b\n"),
			[]codex.Hunk{updateX(codex.Chunk{Old: []string{"a"}, New: []string{"b"}})},
		},
		{
			"header after an update with trailing white space", patch("*** Update File: x\n@@\n-a\n+b\n*** Delete File: y   \n"),
			[]codex.Hunk{
				updateX(codex.Chunk{Old: []string{"a"}, New: []string{"b"}}),
				{Op: model.CandidateDelete, Path: "y"},
			},
		},
		{
			"indented header after an added file", patch("*** Add File: x\n+a\n  *** Delete File: y\n"),
			[]codex.Hunk{
				{Op: model.CandidateCreate, Path: "x", Lines: []string{"a"}},
				{Op: model.CandidateDelete, Path: "y"},
			},
		},
		{
			"end marker repeated after a blank line", "*** Begin Patch\n*** Add File: x\n+a\n*** End Patch\n\n*** End Patch",
			[]codex.Hunk{{Op: model.CandidateCreate, Path: "x", Lines: []string{"a"}}},
		},
		{
			"two carriage returns before a line feed", "*** Begin Patch\r\n*** Add File: x\r\n+a\r\r\n*** End Patch",
			[]codex.Hunk{{Op: model.CandidateCreate, Path: "x", Lines: []string{"a"}}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			hunks, err := codex.ParsePatch(tt.text)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(hunks, tt.want) {
				t.Fatalf("got %+v, want %+v", hunks, tt.want)
			}
		})
	}
}

// The patches the pinned parser refuses stay cannot judge: Codex applies
// none of them.
func TestPatchRefusesWhatThePinnedParserRefuses(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		text string
	}{
		{"tab line after a chunk", patch("*** Update File: x\n@@\n-a\n+b\n\t\n@@\n-c\n+d\n")},
		{"wrapper line with trailing white space", "<<EOF  \n" + patch("*** Add File: x\n+a\n") + "EOF"},
		{"text after a repeated end marker", "*** Begin Patch\n*** End Patch\nx\n*** End Patch"},
		{"change line after the end anchor", patch("*** Update File: x\n@@\n-a\n*** End of File\n-b\n")},
		{"environment line inside an update", patch("*** Update File: x\n@@\n-a\n*** Environment ID: e\n")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := codex.ParsePatch(tt.text); !errors.Is(err, model.ErrCannotJudge) {
				t.Fatalf("want cannot judge, got %v", err)
			}
		})
	}
}

// updateX is the Update File hunk of the file x with its chunks.
func updateX(chunks ...codex.Chunk) codex.Hunk {
	return codex.Hunk{Op: model.CandidateEdit, Path: "x", Chunks: chunks}
}
