package main

import (
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
)

func suggestionLine(language string, enabled []string, suggestions []bootstrap.Suggestion) string {
	if len(suggestions) == 0 {
		return ""
	}
	capabilities := slices.Clone(enabled)
	evidence := make([]string, 0, len(suggestions))
	for _, suggestion := range suggestions {
		capabilities = append(capabilities, suggestion.Capability)
		evidence = append(evidence, suggestion.Capability+" ("+suggestion.Evidence+")")
	}
	slices.Sort(capabilities)
	command := "golaw init --capabilities " + strings.Join(slices.Compact(capabilities), ",")
	if language == model.DefaultLanguage {
		return "Capacidades que os arquivos sugerem, nenhuma habilitada sem pedido: " +
			strings.Join(evidence, ", ") + ". Para habilitá-las: " + command
	}
	return "Capabilities the files suggest, none enabled unless asked: " +
		strings.Join(evidence, ", ") + ". To enable them: " + command
}

// compileSuggestedInitPlan reads the suggestions once the plan compiles, so
// init prints them beside its policy diff and init --update, which compiles
// through compileInitPlan alone, prints none.
func (a *app) compileSuggestedInitPlan(root string, plan initPlan, apply bool) (initPlan, error) {
	plan, err := a.compileInitPlan(root, plan, apply)
	if err != nil {
		return initPlan{}, err
	}
	plan.suggestions, err = bootstrap.Suggest(root, plan.policy.Capabilities)
	return plan, initOperation("suggest capabilities", err)
}
