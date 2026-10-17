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
	}
}

func humanMessage(language string, event model.Event, reason string) string {
	words := sentencesFor(language)
	check, path := reasonTarget(reason, words.unknown)
	if event.Shape == model.ShapePost {
		return fmt.Sprintf(words.post, check, path)
	}
	return fmt.Sprintf(words.blocked, check, path)
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
