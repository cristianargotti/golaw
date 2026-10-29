package surface

import (
	"slices"
	"testing"
)

func TestReadInstructionsJoinsLinesAndSkipsComments(t *testing.T) {
	t.Parallel()
	src := "# syntax=docker/dockerfile:1\nFROM golang:1.27 AS build\nRUN go build \\\n  -o /out \\\n# a note inside the command\n  ./...\n\n# a comment\nenv A=1\nUSER 65532:65532"
	want := []instruction{
		{keyword: "FROM", args: "golang:1.27 AS build", line: 2},
		{keyword: "RUN", args: "go build -o /out ./...", line: 3},
		{keyword: "ENV", args: "A=1", line: 9},
		{keyword: "USER", args: "65532:65532", line: 10},
	}
	if got := readInstructions([]byte(src)); !slices.Equal(got, want) {
		t.Fatalf("instructions = %+v\nwant %+v", got, want)
	}
}
