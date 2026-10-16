package hook

import (
	"fmt"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

const policyFile = "golaw.yml"

type sentences struct {
	blocked string
	post    string
	unknown string
}

// sentencesFor is Portuguese for pt-BR and English for any other language,
// the rule report.Human follows for a decision, so a policy in another
// language never reads Portuguese.
func sentencesFor(language string) sentences {
	if language == model.DefaultLanguage {
		return sentences{
			blocked: "A operação foi bloqueada pela verificação %s no caminho %s.",
			post:    "Corrija a verificação %s no caminho %s antes de continuar.",
			unknown: "não informado",
		}
	}
	return sentences{
		blocked: "The operation was blocked by check %s at path %s.",
		post:    "Fix check %s at path %s before continuing.",
		unknown: "not given",
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
