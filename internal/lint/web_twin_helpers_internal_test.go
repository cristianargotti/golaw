package lint

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/surfacefix"
)

// webTwin decodes a twin of the fixture repository (its .esc files lose
// their suffix and their escapes).
func webTwin(t *testing.T, twin surfacefix.Twin) map[string][]byte {
	t.Helper()
	files, err := surfacefix.Files(twin)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// webTally counts the issues per path and finding code.
func webTally(issues []Issue) map[string]int {
	tally := map[string]int{}
	for _, i := range issues {
		tally[i.Path+" "+Code(i.Rule)]++
	}
	return tally
}

// assertWebTwin holds the comparison of the twin test apart, so the test
// stays under the cognitive limit of the repository: each expectation at its
// count in the red tally and absent from the corrected one.
func assertWebTwin(t *testing.T, expected []surfacefix.Expectation, got, fixed map[string]int) {
	t.Helper()
	for _, e := range expected {
		key := e.Path + " " + e.Code
		if got[key] != e.Count {
			t.Errorf("%s (%s): the red twin yields %d, want %d", key, e.Class, got[key], e.Count)
		}
		if fixed[key] != 0 {
			t.Errorf("%s (%s): the corrected twin still yields %d", key, e.Class, fixed[key])
		}
	}
}
