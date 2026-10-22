package install

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
)

func TestAgentBlockMerge(t *testing.T) {
	t.Parallel()
	cinHome(t)
	begin := "<!-- golaw:begin -->"
	end := "<!-- golaw:end -->"
	block := begin + "\nCurrent instructions.\n" + end + "\n"
	old := begin + "\nOld instructions.\n" + end
	for _, row := range []struct {
		name, current, want string
		invalid             bool
	}{
		{"empty", "", block, false},
		{"append", "Personal instructions.\n", "Personal instructions.\n" + block, false},
		{"separator", "Personal instructions.", "Personal instructions.\n" + block, false},
		{
			"replace", "Before\n" + old + "\nAfter\n",
			"Before\n" + strings.TrimSuffix(block, "\n") + "\nAfter\n", false,
		},
		{"idempotent", block, block, false},
		{"orphan begin", begin, "", true},
		{"orphan end", end, "", true},
		{"reversed", end + "\n" + begin, "", true},
		{"duplicate", old + "\n" + old, "", true},
		{"near marker", "golaw:begin\n", "golaw:begin\n" + block, false},
		{"crlf append", "Personal instructions.\r\n", "Personal instructions.\r\n" + block, false},
		{
			"crlf replace", "Before\r\n" + strings.ReplaceAll(old, "\n", "\r\n") + "\r\nAfter\r\n",
			"Before\r\n" + strings.TrimSuffix(block, "\n") + "\r\nAfter\r\n", false,
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			agentBlockMergeRow(t, row.current, row.want, block, row.invalid)
		})
	}
}

// agentBlockMergeRow holds one row of TestAgentBlockMerge: merged once and
// then again for idempotence, or refused with ErrSettings.
func agentBlockMergeRow(t *testing.T, current, want, block string, invalid bool) {
	t.Helper()
	got, err := mergeAgentBlock([]byte(current), []byte(block))
	if invalid {
		if !errors.Is(err, ErrSettings) {
			t.Fatalf("error = %v", err)
		}
		return
	}
	if err != nil || string(got) != want {
		t.Fatalf("merge = %q, %v; want %q", got, err, want)
	}
	again, err := mergeAgentBlock(got, []byte(block))
	if err != nil || !bytes.Equal(again, got) {
		t.Fatalf("repeat = %q, %v", again, err)
	}
}

func TestAgentBlockBoundsAndSource(t *testing.T) {
	t.Parallel()
	cinHome(t)
	source, err := assets.Read("agents/codex/AGENTS.md")
	must(t, err)
	if len(source) > 1000 ||
		!bytes.Contains(source, []byte("~/.agents/skills/golaw/SKILL.md")) {
		t.Fatalf("source budget or pointer: %q", source)
	}
	begin := "<!-- golaw:begin -->"
	end := "<!-- golaw:end -->"
	for _, size := range []int{1000, 1001} {
		block := begin + strings.Repeat("x", size-len(begin)-len(end)) + end
		_, mergeErr := mergeAgentBlock(nil, []byte(block))
		if (size == 1001) != errors.Is(mergeErr, ErrSettings) {
			t.Fatalf("size %d: %v", size, mergeErr)
		}
	}
	current := append([]byte("Before\n"), source...)
	current = append(current, []byte("After\n")...)
	start, endIndex, found, err := agentBlockSpan(current)
	if err != nil || !found ||
		!bytes.Equal(current[start:endIndex], bytes.TrimSuffix(source, []byte("\n"))) {
		t.Fatalf("span = %d, %d, %v, %v", start, endIndex, found, err)
	}
	for _, invalid := range []string{
		"unmarked",
		"outside\n" + string(source),
		string(source) + "outside\n",
	} {
		if _, mergeErr := mergeAgentBlock(nil, []byte(invalid)); !errors.Is(mergeErr, ErrSettings) {
			t.Fatalf("invalid source accepted: %q: %v", invalid, mergeErr)
		}
	}
}
