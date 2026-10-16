package hook

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// The causes a notice gives the agent: the read bound of a file, the bound
// of an event's input, the deadline and a pre event without a baseline.
const (
	causeReadBound = "it is over the 8 MiB the hook reads"
	causeInput     = "the event is over the 8 MiB the hook reads"
	causeDeadline  = "the hook reached its deadline first"
	causeBaseline  = "its pre event left no baseline"
)

// OverReadBound is the gap of a file over the read bound, which a decoder
// names when it cannot reconstruct a write within the bound.
func OverReadBound(path string) Gap { return Gap{Path: path, Cause: causeReadBound} }

// noticed is the allow of an event that left something unmeasured: the
// notice names each gap and its cause for the agent and leaves it to the
// pipeline, and the human message names the same paths to the person.
func noticed(gaps []Gap, language string) Result {
	return Result{Outcome: Allow, Notice: noticeText(gaps), Human: noticeHuman(gaps, language)}
}

func noticeText(gaps []Gap) string {
	parts := make([]string, 0, len(gaps))
	for _, gap := range gaps {
		subject, object := gap.Path, "it"
		if subject == "" {
			subject, object = "the changes of this command", "them"
		}
		parts = append(parts, fmt.Sprintf("golaw did not measure %s: %s; the pipeline judges %s.", subject, gap.Cause, object))
	}
	return strings.Join(parts, " ")
}

func noticeHuman(gaps []Gap, language string) string {
	words := sentencesFor(language)
	subjects := make([]string, 0, len(gaps))
	for _, gap := range gaps {
		subject := gap.Path
		if subject == "" {
			subject = words.changes
		}
		if !slices.Contains(subjects, subject) {
			subjects = append(subjects, subject)
		}
	}
	return fmt.Sprintf(words.unmeasured, strings.Join(subjects, ", "))
}

// withGaps adds the notice of the gaps to a result: an allow carries it as
// its notice, feedback after a command as more of its reason, so a finding
// still asks for its fix while the agent learns what was not measured.
func withGaps(result Result, gaps []Gap, language string) Result {
	switch {
	case len(gaps) == 0:
		return result
	case result.Outcome == Allow:
		return noticed(gaps, language)
	case result.Outcome == Feedback && !result.CannotJudge:
		result.Reason += " " + noticeText(gaps)
		return result
	default:
		return result
	}
}

// detectionGap names why the changes of a command could not be measured,
// in the words the agent acts on.
func detectionGap(err error) Gap {
	switch {
	case errors.Is(err, model.ErrDeadline):
		return Gap{Cause: causeDeadline}
	case errors.Is(err, fs.ErrNotExist):
		return Gap{Cause: causeBaseline}
	default:
		return Gap{Cause: err.Error()}
	}
}

// measurable keeps the targets a check can read and names the others, a
// text file over the read bound, as gaps.
func measurable(targets []model.File) ([]model.File, []Gap) {
	var gaps []Gap
	kept := targets[:0:0]
	for _, file := range targets {
		if file.Class.IsText() && file.Size > model.MaxReadBytes {
			gaps = append(gaps, OverReadBound(file.Path))
			continue
		}
		kept = append(kept, file)
	}
	return kept, gaps
}
