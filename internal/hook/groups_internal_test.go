package hook

import (
	"strconv"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

// TestTheNoticeGroupsThePathsOfACause names the paths of one cause in one
// sentence, twenty by name and the rest counted, for the agent with the
// cause in the number of its paths and for the person in the language of
// the policy: the changes of the command first in the sentences of their
// cause, a cause of one path as before, the paths of each other cause in
// their own sentence, a path named twice once, and the files a move
// brought last.
func TestTheNoticeGroupsThePathsOfACause(t *testing.T) {
	t.Parallel()
	many := make([]string, 25)
	for i := range many {
		many[i] = "f" + strconv.Itoa(i+10) + ".md"
	}
	gaps := append([]Gap{{Cause: causeMove}}, pathGaps([]string{"c.md"}, causeMove)...)
	gaps = append(gaps, pathGaps(many, causeCommits)...)
	gaps = append(gaps, OverReadBound("big.sql"), OverReadBound("big.sql"))
	gaps = append(gaps, pathGaps([]string{"a.md", "b.md"}, causeConvert)...)
	gaps = append(gaps, pathGaps([]string{"x.md", "y.md"}, causeUnmerged)...)
	gaps = append(gaps, pathGaps([]string{"moved.md"}, causeArrived)...)
	listed := strings.Join(many[:20], ", ")
	text := "golaw did not measure the changes of this command: git could not compare the HEAD the command started from with the HEAD it left; the pipeline judges them. " +
		"golaw did not measure c.md: git could not compare the HEAD the command started from with the HEAD it left; the pipeline judges it. " +
		"golaw did not measure " + listed + " and 5 more: git could not tell which commits this command created; the pipeline judges them. " +
		"golaw did not measure big.sql: it is over the 8 MiB the hook reads; the pipeline judges it. " +
		"golaw did not measure a.md, b.md: git could not convert them to compare; the pipeline judges them. " +
		"golaw did not measure x.md, y.md: git left them unmerged, and golaw judges the resolutions the agent writes; the pipeline judges them. " +
		"golaw did not judge the files the move of HEAD brought from commits this command did not create, which ask for no fix: moved.md; the pipeline judges them."
	human := "golaw n\u00e3o mediu as mudan\u00e7as deste comando, c.md, que o pipeline julga. " +
		"golaw n\u00e3o mediu " + listed + " e mais 5, que o pipeline julga. " +
		"golaw n\u00e3o mediu big.sql, que o pipeline julga. " +
		"golaw n\u00e3o mediu a.md, b.md, que o pipeline julga. " +
		"golaw n\u00e3o mediu x.md, y.md, que o pipeline julga. " +
		"golaw n\u00e3o julgou os arquivos que a mudan\u00e7a de HEAD trouxe de commits que este comando n\u00e3o criou, que n\u00e3o pedem corre\u00e7\u00e3o: moved.md; o pipeline os julga."
	if got := noticed(gaps, model.DefaultLanguage); got.Notice != text || got.Human != human {
		t.Fatalf("notice %q, human %q", got.Notice, got.Human)
	}
}

// TestTheNoticeOfManyPathsIsBounded names the paths of one cause twenty by
// name and the rest counted in every channel: the human message in English
// for a policy in another language, the notice of 600 paths, as a move git
// cannot classify leaves, in one sentence for the agent and one for the
// person, and the same sentence appended to feedback.
func TestTheNoticeOfManyPathsIsBounded(t *testing.T) {
	t.Parallel()
	many := make([]string, 25)
	for i := range many {
		many[i] = "f" + strconv.Itoa(i+10) + ".md"
	}
	listed := strings.Join(many[:20], ", ")
	english := noticed(pathGaps(many, causeCommits), "en").Human
	if english != "golaw did not measure "+listed+" and 5 more, which the pipeline judges." {
		t.Fatalf("human in English %q", english)
	}
	thousands := make([]string, 0, 600)
	for i := range 600 {
		thousands = append(thousands, "files/f"+strconv.Itoa(i)+".md")
	}
	bounded := noticed(pathGaps(thousands, causeCommits), model.DefaultLanguage)
	if strings.Count(bounded.Notice, "golaw did not measure") != 1 || len(bounded.Notice) > 600 || len(bounded.Human) > 600 ||
		!strings.Contains(bounded.Notice, " and 580 more: ") || !strings.Contains(bounded.Human, " e mais 580, ") {
		t.Fatalf("600 paths of one cause: notice %q, human %q", bounded.Notice, bounded.Human)
	}
	feedback := withGaps(Result{Outcome: Feedback, Reason: "text.dashes: a dash (mine.md:1)"}, pathGaps(thousands, causeCommits), model.DefaultLanguage)
	if strings.Count(feedback.Reason, "golaw did not measure") != 1 || len(feedback.Reason) > 700 {
		t.Fatalf("feedback with 600 paths of one cause: %q", feedback.Reason)
	}
}
