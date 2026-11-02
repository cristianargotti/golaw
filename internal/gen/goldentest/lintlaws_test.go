package goldentest_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/goldentest"
)

// The extras of the lint laws: the shapes the Sonar green record had to
// rewrite in generated code (M7.10.2), each beside the shape that passes.
const (
	lintS4144IdenticalMethodsRed = `
// Spread answers how far the values spread.
func (g *TYPE) Spread(values []int) (int, int, int) {
	low, high, zeros := 0, 0, 0
	for index, value := range values {
		switch {
		case value < low:
			low = value
		case value > high:
			high = value
		case value == 0:
			zeros += index
		}
		if low < -1000 || high > 1000 {
			return low, high, zeros
		}
	}
	if low == 0 && high == 0 {
		return len(g.prefix), len(values), zeros
	}
	return low + zeros, high - zeros, len(g.prefix)
}

// Range answers how far the values range.
func (g *TYPE) Range(values []int) (int, int, int) {
	low, high, zeros := 0, 0, 0
	for index, value := range values {
		switch {
		case value < low:
			low = value
		case value > high:
			high = value
		case value == 0:
			zeros += index
		}
		if low < -1000 || high > 1000 {
			return low, high, zeros
		}
	}
	if low == 0 && high == 0 {
		return len(g.prefix), len(values), zeros
	}
	return low + zeros, high - zeros, len(g.prefix)
}
`
	lintS4144IdenticalMethodsNear = `
// Spread answers how far the values spread.
func (g *TYPE) Spread(values []int) (int, int, int) {
	low, high, zeros := 0, 0, 0
	for index, value := range values {
		switch {
		case value < low:
			low = value
		case value > high:
			high = value
		case value == 0:
			zeros += index
		}
		if low < -1000 || high > 1000 {
			return low, high, zeros
		}
	}
	if low == 0 && high == 0 {
		return len(g.prefix), len(values), zeros
	}
	return low + zeros, high - zeros, len(g.prefix)
}

// Range answers the prefix length and the count of values.
func (g *TYPE) Range(values []int) (int, int) {
	return len(g.prefix), len(values)
}
`
	lintS3776CognitiveRed = `
// Rank ranks the values of one account through nested rules.
func (g *TYPE) Rank(values []int) int {
	total := len(g.prefix)
	for _, value := range values {
		if value > 0 {
			for index := range value {
				if index%2 == 0 {
					if index > 1 && index < 100 {
						total += index
					}
				}
			}
		}
	}
	return total
}
`
	lintS3776CognitiveNear = `
// Rank adds the positive values of one account to the prefix length.
func (g *TYPE) Rank(values []int) int {
	total := len(g.prefix)
	for _, value := range values {
		if value > 0 {
			total += value
		}
	}
	return total
}
`
	lintS117NamingRed = `
// Line answers the prefixed line of one user.
func (g *TYPE) Line(name string) string {
	user_name := g.prefix + name
	return user_name
}
`
	lintS117NamingNear = `
// Line answers the prefixed line of one user.
func (g *TYPE) Line(name string) string {
	userName := g.prefix + name
	return userName
}
`
	lintErrcheckBlankRed = `
// Measure answers the size of a name, dropping the measure's error.
func Measure(measure func(string) (int, error), name string) int {
	size, _ := measure(name)
	return size
}
`
	lintErrcheckBlankNear = `
// Measure answers the size of a name and the measure's error.
func Measure(measure func(string) (int, error), name string) (int, error) {
	return measure(name)
}
`
	lintIfaceUnusedRed = `
// Speaker speaks one line.
type Speaker interface {
	Speak() string
}
`
	lintIfaceUnusedNear = `
// Speaker speaks one line.
type Speaker interface {
	Speak() string
}

// Say answers the line of a speaker.
func Say(s Speaker) string {
	return s.Speak()
}
`
)

func TestExecLintFailsEachLawTheRecordRewrote(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		extra, want string // want is a fragment of the lint failure; empty for none
	}{
		"red: S4144-identical-methods, two methods of one type with one body, the shape of the fake over the recorder": {extra: lintS4144IdenticalMethodsRed, want: "(dupl)"},
		"near: S4144-identical-methods":                                   {extra: lintS4144IdenticalMethodsNear},
		"red: S3776-cognitive, a function at cognitive complexity 16":     {extra: lintS3776CognitiveRed, want: "(gocognit)"},
		"near: S3776-cognitive":                                           {extra: lintS3776CognitiveNear},
		"red: S117-naming, a snake case local":                            {extra: lintS117NamingRed, want: "var-naming"},
		"near: S117-naming":                                               {extra: lintS117NamingNear},
		"red: errcheck-blank, an error dropped into the blank identifier": {extra: lintErrcheckBlankRed, want: "Error return value is not checked (errcheck)"},
		"near: errcheck-blank":                                            {extra: lintErrcheckBlankNear},
		"red: iface-unused, an interface the package never uses":          {extra: lintIfaceUnusedRed, want: "(iface)"},
		"near: iface-unused":                                              {extra: lintIfaceUnusedNear},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			report, failures := goldentest.Exec(t, slice(tc.extra, bothFills()...), goldentest.StepLint)
			if reason, skipped := report.Skipped[goldentest.StepLint]; skipped {
				t.Skip(reason)
			}
			joined := strings.Join(failures, "\n")
			if tc.want == "" && len(failures) != 0 {
				t.Fatalf("failures = %s, want none", joined)
			}
			if tc.want != "" && !strings.Contains(joined, tc.want) {
				t.Fatalf("failures = %s, want one holding %q", joined, tc.want)
			}
		})
	}
}
