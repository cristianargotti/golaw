package goast_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// input builds the scan input of a check over an in-memory tree under the
// proof policy and the check's default options.
func input(c model.Check, files map[string]string) model.Input {
	content := make(map[string][]byte, len(files))
	for p, text := range files {
		content[p] = []byte(text)
	}
	tree := proof.NewTree(content)
	return model.Input{
		Mode:     model.ModeScan,
		Policy:   proof.Policy(),
		Snapshot: tree,
		Targets:  tree.Files(),
		Options:  c.Spec().Options.Defaults(),
	}
}

// scan runs one check over the files and fails the test on an error.
func scan(t *testing.T, c model.Check, files map[string]string) model.Result {
	t.Helper()
	res, err := c.Run(t.Context(), input(c, files))
	if err != nil {
		t.Fatalf("%s: %v", c.Spec().ID, err)
	}
	return res
}

func codes(res model.Result) []string {
	out := make([]string, 0, len(res.Findings))
	for k := range res.Findings {
		f := &res.Findings[k]
		out = append(out, f.Code)
	}
	return out
}

func lines(res model.Result) []int {
	out := make([]int, 0, len(res.Findings))
	for k := range res.Findings {
		f := &res.Findings[k]
		out = append(out, f.Line)
	}
	return out
}

// expectCodes fails unless the result is measured with exactly these codes.
func expectCodes(t *testing.T, res model.Result, want ...string) {
	t.Helper()
	if res.Status.Kind != model.StatusMeasured {
		t.Fatalf("status = %s (%s), want measured", res.Status.Kind, res.Status.Reason)
	}
	if got := codes(res); !slices.Equal(got, want) && (len(got) > 0 || len(want) > 0) {
		t.Fatalf("codes = %v, want %v", got, want)
	}
}

// expectLines fails unless the findings sit on these lines in order.
func expectLines(t *testing.T, res model.Result, want ...int) {
	t.Helper()
	if got := lines(res); !slices.Equal(got, want) {
		t.Fatalf("lines = %v, want %v", got, want)
	}
}

// expectUnmeasured fails unless the status is unmeasured for a reason that
// carries the text.
func expectUnmeasured(t *testing.T, res model.Result, text string) {
	t.Helper()
	if res.Status.Kind != model.StatusUnmeasured || !strings.Contains(res.Status.Reason, text) {
		t.Fatalf("status = %s (%s), want unmeasured with %q", res.Status.Kind, res.Status.Reason, text)
	}
}
