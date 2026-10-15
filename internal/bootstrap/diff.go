package bootstrap

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
)

// Diff renders the change from old to current as a unified diff that shows
// every line once, prefixed with a space, a minus or a plus, so the person
// reads the whole file init proposes. It is empty when the contents are
// equal, which is how "init --apply twice yields no diff" is observed.
func Diff(old, current []byte) string {
	if bytes.Equal(old, current) {
		return ""
	}
	before, after := lines(old), lines(current)
	from := "--- " + PolicyFile
	if old == nil {
		from = "--- /dev/null"
	}
	header := []string{from, "+++ " + PolicyFile, fmt.Sprintf("@@ -1,%d +1,%d @@", len(before), len(after))}
	out := make([]string, 0, len(before)+len(after)+len(header))
	out = append(out, header...)
	out = append(out, edits(before, after)...)
	return strings.Join(out, "\n") + "\n"
}

// lines splits content into lines without a trailing empty line.
func lines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// edits walks the longest common subsequence of the two line lists and
// emits every line with its prefix, keeping the order of both sides.
func edits(before, after []string) []string {
	table := lcsTable(before, after)
	out := make([]string, 0, len(before)+len(after))
	i, j := 0, 0
	for i < len(before) && j < len(after) {
		switch {
		case before[i] == after[j]:
			out = append(out, " "+before[i])
			i++
			j++
		case table[i+1][j] >= table[i][j+1]:
			out = append(out, "-"+before[i])
			i++
		default:
			out = append(out, "+"+after[j])
			j++
		}
	}
	for ; i < len(before); i++ {
		out = append(out, "-"+before[i])
	}
	for ; j < len(after); j++ {
		out = append(out, "+"+after[j])
	}
	return out
}

// lcsTable holds, at [i][j], the length of the longest common subsequence
// of before[i:] and after[j:]; the policy file is short, so the quadratic
// table is cheaper than anything cleverer.
func lcsTable(before, after []string) [][]int {
	table := make([][]int, len(before)+1)
	for i := range table {
		table[i] = make([]int, len(after)+1)
	}
	for i := range slices.Backward(before) {
		for j := range slices.Backward(after) {
			if before[i] == after[j] {
				table[i][j] = table[i+1][j+1] + 1
			} else {
				table[i][j] = max(table[i+1][j], table[i][j+1])
			}
		}
	}
	return table
}
