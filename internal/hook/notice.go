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
// of an event's input, the deadline, a pre event without a baseline, a move
// of HEAD git could not compare and commits git could not tell apart. A
// file a move brought from a commit the command did not create has a
// sentence of its own, causeArrived marks it.
const (
	causeReadBound = "it is over the 8 MiB the hook reads"
	causeInput     = "the event is over the 8 MiB the hook reads"
	causeDeadline  = "the hook reached its deadline first"
	causeBaseline  = "its pre event left no baseline"
	causeMove      = "git could not compare the HEAD the command started from with the HEAD it left"
	causeCommits   = "git could not tell which commits this command created"
	causeArrived   = "the move of HEAD brought it from a commit this command did not create"
)

// maxArrived is how many of the files a move brought a notice names before
// it counts the rest.
const maxArrived = 20

// arrivedText is the sentence of the files a move brought, for the agent.
const arrivedText = "golaw did not judge the files the move of HEAD brought from commits this command did not create, which ask for no fix: %s; the pipeline judges them."

// OverReadBound is the gap of a file over the read bound, which a decoder
// names when it cannot reconstruct a write within the bound.
func OverReadBound(path string) Gap { return Gap{Path: path, Cause: causeReadBound} }

// pathGaps is the gap of each path for one cause.
func pathGaps(paths []string, cause string) []Gap {
	gaps := make([]Gap, 0, len(paths))
	for _, path := range paths {
		gaps = append(gaps, Gap{Path: path, Cause: cause})
	}
	return gaps
}

// OverInputBound is the gap of a write whose event is over the bound of the
// input, which the decoder of such an event names in place of its content.
func OverInputBound(path string) Gap { return Gap{Path: path, Cause: causeInput} }

// noticed is the allow of an event that left something unmeasured: the
// notice names each gap and its cause for the agent and leaves it to the
// pipeline, and the human message names the same paths to the person.
func noticed(gaps []Gap, language string) Result {
	return Result{Outcome: Allow, Notice: noticeText(gaps), Human: noticeHuman(gaps, language)}
}

func noticeText(gaps []Gap) string {
	unmeasured, arrived := splitArrived(gaps)
	parts := make([]string, 0, len(unmeasured)+1)
	for _, gap := range unmeasured {
		subject, object := gap.Path, "it"
		if subject == "" {
			subject, object = "the changes of this command", "them"
		}
		parts = append(parts, fmt.Sprintf("golaw did not measure %s: %s; the pipeline judges %s.", subject, gap.Cause, object))
	}
	if len(arrived) > 0 {
		parts = append(parts, fmt.Sprintf(arrivedText, namedPaths(arrived, sentencesFor("").more)))
	}
	return strings.Join(parts, " ")
}

func noticeHuman(gaps []Gap, language string) string {
	words := sentencesFor(language)
	unmeasured, arrived := splitArrived(gaps)
	subjects := make([]string, 0, len(unmeasured))
	for _, gap := range unmeasured {
		subject := gap.Path
		if subject == "" {
			subject = words.changes
		}
		if !slices.Contains(subjects, subject) {
			subjects = append(subjects, subject)
		}
	}
	parts := make([]string, 0, 2)
	if len(subjects) > 0 {
		parts = append(parts, fmt.Sprintf(words.unmeasured, strings.Join(subjects, ", ")))
	}
	if len(arrived) > 0 {
		parts = append(parts, fmt.Sprintf(words.arrived, namedPaths(arrived, words.more)))
	}
	return strings.Join(parts, " ")
}

// splitArrived parts the gaps of what was not measured from the paths of
// the files a move brought, which a notice names in one sentence.
func splitArrived(gaps []Gap) ([]Gap, []string) {
	unmeasured := make([]Gap, 0, len(gaps))
	var arrived []string
	for _, gap := range gaps {
		if gap.Cause == causeArrived {
			arrived = append(arrived, gap.Path)
			continue
		}
		unmeasured = append(unmeasured, gap)
	}
	return unmeasured, arrived
}

// namedPaths names up to maxArrived paths and counts the rest in the words
// of more.
func namedPaths(paths []string, more string) string {
	if len(paths) <= maxArrived {
		return strings.Join(paths, ", ")
	}
	return strings.Join(paths[:maxArrived], ", ") + fmt.Sprintf(more, len(paths)-maxArrived)
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
