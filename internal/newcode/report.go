package newcode

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	percent      = 100
	tenths       = 10
	maxUncovered = 20
)

// Percent answers the covered share of the new lines to cover, 0 when there
// is none, so an empty change never reads as fully covered.
func (r Report) Percent() float64 {
	if r.ToCover == 0 {
		return 0
	}
	return float64(r.Covered) * percent / float64(r.ToCover)
}

// Passed compares in integers so the decision never depends on rounding.
func (r Report) Passed() bool {
	return r.Status.Kind == model.StatusMeasured && r.Covered*percent >= r.Threshold*r.ToCover
}

// Text answers the lines a person reads for the report.
func (r Report) Text() []string {
	switch r.Status.Kind {
	case model.StatusMeasured:
		return r.measuredText()
	case model.StatusNotApplicable:
		return []string{"new code: not applicable, " + r.Status.Reason}
	case model.StatusUnmeasured:
		return []string{"new code: unmeasured, " + r.Status.Reason}
	}
	return []string{"new code: unmeasured, the report carries no status"}
}

func (r Report) measuredText() []string {
	// Floored, never rounded, so a ratio under the threshold never prints
	// as the threshold itself.
	shown := math.Floor(r.Percent()*tenths) / tenths
	out := []string{fmt.Sprintf("new code: %.1f%% (%d/%d new lines covered), threshold %d%%",
		shown, r.Covered, r.ToCover, r.Threshold)}
	for _, file := range r.Files {
		out = append(out, fileLine(file))
	}
	if r.Passed() {
		return append(out, "verdict: at or above the threshold")
	}
	return append(out, "verdict: below the threshold, cover the uncovered new lines")
}

func fileLine(file File) string {
	line := fmt.Sprintf("%s: %d/%d covered", file.Path, file.Covered, file.ToCover)
	if len(file.Uncovered) == 0 {
		return line
	}
	shown := file.Uncovered[:min(len(file.Uncovered), maxUncovered)]
	numbers := make([]string, 0, len(shown))
	for _, number := range shown {
		numbers = append(numbers, strconv.Itoa(number))
	}
	line += ", uncovered lines " + strings.Join(numbers, ", ")
	if rest := len(file.Uncovered) - len(shown); rest > 0 {
		line += fmt.Sprintf(", and %d more", rest)
	}
	return line
}
