package codex_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/codex"
)

func TestApplyChunks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		before  string
		chunks  []codex.Chunk
		want    string
		applied bool
	}{
		{
			"replace", "a\nb\nc\n",
			[]codex.Chunk{{Old: []string{"b"}, New: []string{"B"}}},
			"a\nB\nc\n", true,
		},
		{
			"absent", "a\n",
			[]codex.Chunk{{Old: []string{"b"}, New: []string{"B"}}},
			"", false,
		},
		{
			"ambiguous", "a\na\n", //nolint:dupword // two equal lines make the match ambiguous
			[]codex.Chunk{{Old: []string{"a"}, New: []string{"A"}}},
			"", false,
		},
		{
			"whitespace", " a \n",
			[]codex.Chunk{{Old: []string{"a"}, New: []string{"A"}}},
			"", false,
		},
		{
			"context", "a\nheading\na\n",
			[]codex.Chunk{{Context: "heading", Old: []string{"a"}, New: []string{"A"}}},
			"a\nheading\nA\n", true,
		},
		{
			"missing-context", "a\n",
			[]codex.Chunk{{Context: "heading", Old: []string{"a"}, New: []string{"A"}}},
			"", false,
		},
		{
			"ambiguous-context", "heading\na\nheading\na\n",
			[]codex.Chunk{{Context: "heading", Old: []string{"a"}, New: []string{"A"}}},
			"", false,
		},
		{
			"eof", "a\nb\n",
			[]codex.Chunk{{Old: []string{"b"}, New: []string{"B"}, EOF: true}},
			"a\nB\n", true,
		},
		{
			"not-eof", "a\nb\n",
			[]codex.Chunk{{Old: []string{"a"}, New: []string{"A"}, EOF: true}},
			"", false,
		},
		{"ordered", "a\nb\nc\n", []codex.Chunk{
			{Old: []string{"a"}, New: []string{"A"}},
			{Old: []string{"c"}, New: []string{"C"}},
		}, "A\nb\nC\n", true},
		{"backwards", "a\nb\n", []codex.Chunk{
			{Old: []string{"b"}, New: []string{"B"}},
			{Old: []string{"a"}, New: []string{"A"}},
		}, "", false},
		{"new-content-is-not-base", "a\n", []codex.Chunk{
			{Old: []string{"a"}, New: []string{"b"}},
			{Old: []string{"b"}, New: []string{"c"}},
		}, "", false},
		{
			"append", "a\n",
			[]codex.Chunk{{New: []string{"b"}}},
			"a\nb\n", true,
		},
		{
			"empty-append", "",
			[]codex.Chunk{{New: []string{"b"}}},
			"b\n", true,
		},
		{
			"delete", "a\nb\n",
			[]codex.Chunk{{Old: []string{"b"}}},
			"a\n", true,
		},
		{
			"longer-old", "a\n",
			[]codex.Chunk{{Old: []string{"a", "b"}}},
			"", false,
		},
		{
			"unterminated", "a",
			[]codex.Chunk{{Old: []string{"a"}, New: []string{"b"}}},
			"b\n", true,
		},
		{
			"crlf", "a\r\nb\r\n",
			[]codex.Chunk{{Old: []string{"b"}, New: []string{"B"}}},
			"a\r\nB\r\n", true,
		},
		{
			"mixed-endings", "a\r\nb\n",
			[]codex.Chunk{{Old: []string{"b"}, New: []string{"B"}}},
			"", false,
		},
		{"no-chunks", "a", nil, "a", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, applied := codex.ApplyChunks([]byte(tt.before), tt.chunks)
			if applied != tt.applied {
				t.Fatalf("applied = %v, want %v", applied, tt.applied)
			}
			if applied && string(got) != tt.want {
				t.Fatalf("content = %q, want %q", got, tt.want)
			}
		})
	}
}
