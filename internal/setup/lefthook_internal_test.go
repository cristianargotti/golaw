package setup

import (
	"slices"
	"strings"
	"testing"
)

// TestRunValuesReadsOnlyTheRunKeys reads the run value of every command or
// job in each YAML form lefthook accepts, without its quoting, and leaves
// out every other key, quoted text that holds run: among them. Only the
// shell words of a value count, so the comparison folds whitespace.
func TestRunValuesReadsOnlyTheRunKeys(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		body string
		want []string
	}{
		"plain":         {"\n  commands:\n    a:\n      run: golaw hook pre-commit\n", []string{"golaw hook pre-commit"}},
		"single quoted": {"\n  commands:\n    a:\n      run: 'it''s \"golaw\"'\n", []string{`it's "golaw"`}},
		"double quoted": {"\n  commands:\n    a:\n      run: \"C:\\\\bin\\\\golaw.exe\\thook \\\"x\\\"\"\n", []string{`C:\bin\golaw.exe hook "x"`}},
		"unterminated":  {"\n  commands:\n    a:\n      run: \"golaw hook", []string{"golaw hook"}},
		"anchor and tag": {
			"\n  commands:\n    a:\n      run: &rung !!str golaw hook pre-commit\n",
			[]string{"golaw hook pre-commit"},
		},
		"block literal": {
			"\n  commands:\n    a:\n      run: |-\n        make vet\n\n        golaw hook pre-commit\n      fail_text: no\n",
			[]string{"make vet golaw hook pre-commit"},
		},
		"plain continued": {
			"\n  jobs:\n    - run: golaw hook\n        pre-commit\n      name: golaw\n",
			[]string{"golaw hook pre-commit"},
		},
		"flow": {
			" {commands: {a: {run: golaw hook pre-commit, tags: [a, b]}, b: {'run': 'make x'}}}\n",
			[]string{"golaw hook pre-commit", "make x"},
		},
		"other keys": {
			"\n  commands:\n    a:\n      name: 'run: golaw hook pre-commit'\n      fail_text: don't run it\n      run: make\n",
			[]string{"make"},
		},
		"a quoted key that is not run": {"\n  commands:\n    a:\n      \"name\": golaw\n      run: make\n", []string{"make"}},
		"no run":                       {"\n  commands:\n    a:\n      rung: x\n      xrun: y\n      run:x\n", nil},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := folded(runValues(tc.body)); !slices.Equal(got, tc.want) {
				t.Fatalf("runValues = %q, want %q", got, tc.want)
			}
		})
	}
}

// folded joins the fields of each value with one space.
func folded(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, strings.Join(strings.Fields(value), " "))
	}
	return out
}

// TestRoutedSplitsShellWords accepts golaw by name or path, quoted or not,
// after a shell operator, and refuses golaw words that are not hook <rung>.
func TestRoutedSplitsShellWords(t *testing.T) {
	t.Parallel()
	for body, want := range map[string]bool{
		"\n      run: '\"golaw\" hook pre-commit'\n":                   true,
		"\n      run: cd sub&&'/opt/my tools/golaw' hook pre-commit\n": true,
		"\n      run: \"C:\\\\tools\\\\golaw.exe hook pre-commit\"\n":  true,
		"\n      run: sh -c 'golaw hook pre-commit'\n":                 true,
		"\n      run: golaw hook pre-commits\n":                        false,
		"\n      run: golaw-old hook pre-commit\n":                     false,
		"\n      run: make\n      fail_text: golaw hook pre-commit\n":  false,
	} {
		if got := routed(body, "pre-commit"); got != want {
			t.Errorf("routed(%q) = %v, want %v", body, got, want)
		}
	}
}
