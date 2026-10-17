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
// of HEAD git could not compare, commits git could not tell apart, bytes
// git could not convert to compare them, a conflict git left unmerged, and
// a merge in progress and a merge commit of the command git could not make
// by itself. A file a move or a merge brought from a commit the command did
// not create has a sentence of its own, causeArrived and causeMerged mark
// it.
const (
	causeReadBound      = "it is over the 8 MiB the hook reads"
	causeInput          = "the event is over the 8 MiB the hook reads"
	causeDeadline       = "the hook reached its deadline first"
	causeBaseline       = "its pre event left no baseline"
	causeMove           = "git could not compare the HEAD the command started from with the HEAD it left"
	causeCommits        = "git could not tell which commits this command created"
	causeConvert        = "git could not convert it to compare"
	causeUnmerged       = "git left it unmerged, and golaw judges the resolution the agent writes"
	causeMergeUnknown   = "git could not make the merge in progress by itself to compare it"
	causeRemergeUnknown = "git could not make the merge commit of this command by itself to compare it"
	causeArrived        = "the move of HEAD brought it from a commit this command did not create"
	causeMerged         = "the merge in progress brought it from a commit this command did not create"
)

// maxNamed is how many paths a sentence of a notice names before it counts
// the rest, for every cause and for the files a move or a merge brought.
const maxNamed = 20

// arrivedText and mergedText are the sentences of the files a move of HEAD
// and a merge in progress brought, for the agent.
const (
	arrivedText = "golaw did not judge the files the move of HEAD brought from commits this command did not create, which ask for no fix: %s; the pipeline judges them."
	mergedText  = "golaw did not judge the files the merge in progress brought from commits this command did not create, which ask for no fix: %s; the pipeline judges them."
)

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
// notice names the paths of each cause in one sentence with that cause for
// the agent and leaves them to the pipeline, and the human message names
// the same paths to the person, each sentence naming twenty and counting
// the rest.
func noticed(gaps []Gap, language string) Result {
	return Result{Outcome: Allow, Notice: noticeText(gaps), Human: noticeHuman(gaps, language)}
}

func noticeText(gaps []Gap) string {
	groups, files := groupGaps(gaps)
	more := sentencesFor("").more
	parts := make([]string, 0, 2*len(groups)+2)
	for _, g := range groups {
		if g.whole {
			parts = append(parts, fmt.Sprintf("golaw did not measure the changes of this command: %s; the pipeline judges them.", g.cause))
		}
		if len(g.paths) > 0 {
			object := "it"
			if len(g.paths) > 1 {
				object = "them"
			}
			parts = append(parts, fmt.Sprintf("golaw did not measure %s: %s; the pipeline judges %s.", namedPaths(g.paths, more), causeOf(g.cause, len(g.paths)), object))
		}
	}
	return strings.Join(append(parts, files.sentences(arrivedText, mergedText, more)...), " ")
}

func noticeHuman(gaps []Gap, language string) string {
	words := sentencesFor(language)
	groups, files := groupGaps(gaps)
	var parts []string
	for _, g := range groups {
		var subjects []string
		if g.whole {
			subjects = append(subjects, words.changes)
		}
		if len(g.paths) > 0 {
			subjects = append(subjects, namedPaths(g.paths, words.more))
		}
		if sentence := fmt.Sprintf(words.unmeasured, strings.Join(subjects, ", ")); !slices.Contains(parts, sentence) {
			parts = append(parts, sentence)
		}
	}
	return strings.Join(append(parts, files.sentences(words.arrived, words.merged, words.more)...), " ")
}

// group is what a notice says of one cause: whether a gap of it names the
// changes of the command, and the paths its gaps name, each once, in the
// order they came.
type group struct {
	cause string
	whole bool
	paths []string
}

// groupGaps gathers the gaps of what was not measured by cause, in the
// order each cause first came, so a cause of many paths is one sentence,
// and parts out the files a move or a merge brought.
func groupGaps(gaps []Gap) ([]group, brought) {
	var groups []group
	var files brought
	at := map[string]int{}
	named := map[[2]string]bool{}
	for _, gap := range gaps {
		switch gap.Cause {
		case causeArrived:
			files.moved = append(files.moved, gap.Path)
			continue
		case causeMerged:
			files.merged = append(files.merged, gap.Path)
			continue
		}
		i, seen := at[gap.Cause]
		if !seen {
			i, at[gap.Cause] = len(groups), len(groups)
			groups = append(groups, group{cause: gap.Cause})
		}
		key := [2]string{gap.Cause, gap.Path}
		switch {
		case gap.Path == "":
			groups[i].whole = true
		case !named[key]:
			named[key] = true
			groups[i].paths = append(groups[i].paths, gap.Path)
		}
	}
	return groups, files
}

// causeOf is a cause in the words that name as many files as it has paths:
// the causes that say "it" of one file say "them" of several.
func causeOf(cause string, paths int) string {
	if paths < 2 {
		return cause
	}
	switch cause {
	case causeReadBound:
		return "they are over the 8 MiB the hook reads"
	case causeConvert:
		return "git could not convert them to compare"
	case causeUnmerged:
		return "git left them unmerged, and golaw judges the resolutions the agent writes"
	default:
		return cause
	}
}

// brought are the files a command brought from commits it did not create:
// those of a move of HEAD and those of a merge in progress, which a notice
// names in a sentence each.
type brought struct {
	moved, merged []string
}

// sentences names the files of a move and then those of a merge in the
// sentences given, each only when it has a file.
func (b brought) sentences(moved, merged, more string) []string {
	var parts []string
	if len(b.moved) > 0 {
		parts = append(parts, fmt.Sprintf(moved, namedPaths(b.moved, more)))
	}
	if len(b.merged) > 0 {
		parts = append(parts, fmt.Sprintf(merged, namedPaths(b.merged, more)))
	}
	return parts
}

// namedPaths names up to maxNamed paths and counts the rest in the words
// of more.
func namedPaths(paths []string, more string) string {
	if len(paths) <= maxNamed {
		return strings.Join(paths, ", ")
	}
	return strings.Join(paths[:maxNamed], ", ") + fmt.Sprintf(more, len(paths)-maxNamed)
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
