package hook

import (
	"fmt"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

const policyFile = "golaw.yml"

type sentences struct {
	blocked    string
	post       string
	unknown    string
	unmeasured string
	changes    string
	arrived    string
	merged     string
	more       string
	deadline   string
	oversized  string
	unread     string
}

// sentencesFor is Portuguese for pt-BR and English for any other language,
// the rule report.Human follows for a decision, so a policy in another
// language never reads Portuguese.
func sentencesFor(language string) sentences {
	if language == model.DefaultLanguage {
		return sentences{
			blocked:    "A operação foi bloqueada pela verificação %s no caminho %s.",
			post:       "Corrija a verificação %s no caminho %s antes de continuar.",
			unknown:    "não informado",
			unmeasured: "golaw não mediu %s, que o pipeline julga.",
			changes:    "as mudanças deste comando",
			arrived:    "golaw não julgou os arquivos que a mudança de HEAD trouxe de commits que este comando não criou, que não pedem correção: %s; o pipeline os julga.",
			merged:     "golaw não julgou os arquivos que o merge em andamento trouxe de commits que este comando não criou, que não pedem correção: %s; o pipeline os julga.",
			more:       " e mais %d",
			deadline:   "golaw não terminou de julgar a operação dentro do prazo interno; tente de novo.",
			oversized:  "golaw não julgou a operação porque ela passa dos 8 MiB que o hook lê; divida-a em partes menores.",
			unread:     "golaw não conseguiu ler o que precisa para julgar a operação; o motivo enviado ao agente nomeia a causa.",
		}
	}
	return sentences{
		blocked:    "The operation was blocked by check %s at path %s.",
		post:       "Fix check %s at path %s before continuing.",
		unknown:    "not given",
		unmeasured: "golaw did not measure %s, which the pipeline judges.",
		changes:    "the changes of this command",
		arrived:    arrivedText,
		merged:     mergedText,
		more:       " and %d more",
		deadline:   "golaw did not finish judging the operation within its internal deadline; try again.",
		oversized:  "golaw did not judge the operation because it is over the 8 MiB the hook reads; split it into smaller parts.",
		unread:     "golaw could not read what it needs to judge the operation; the reason given to the agent names the cause.",
	}
}

// humanMessage is the sentence a person reads beside a denial. A judgment
// that stopped before any check names its cause class instead of a check
// and a path it does not have, so the person knows what to change.
func humanMessage(language string, event model.Event, reason string, cannot bool) string {
	words := sentencesFor(language)
	check, path := reasonTarget(reason, words.unknown)
	if cannot && check == "golaw" {
		return cannotJudgeSentence(words, reason)
	}
	if event.Shape == model.ShapePost {
		return fmt.Sprintf(words.post, check, path)
	}
	return fmt.Sprintf(words.blocked, check, path)
}

// cannotJudgeSentence is the cause class of a cannot-judge reason: the
// internal deadline, an input over the 8 MiB the hook reads, or an input
// golaw could not read.
func cannotJudgeSentence(words sentences, reason string) string {
	switch {
	case strings.Contains(reason, model.ErrDeadline.Error()):
		return words.deadline
	case strings.Contains(reason, "8 MiB"):
		return words.oversized
	default:
		return words.unread
	}
}

func reasonTarget(reason, unknown string) (string, string) {
	check, _, found := strings.Cut(reason, ":")
	if !found || check == "" || strings.ContainsAny(check, " \t/()") {
		check = "golaw"
	}
	path := unknown
	if strings.Contains(reason, policyFile) {
		path = policyFile
	}
	if _, after, found := strings.CutLast(reason, " ("); found {
		if candidate, ok := strings.CutSuffix(after, ")"); ok {
			path = candidate
		}
	}
	return check, path
}
