package bootstrap

import (
	"fmt"
	"io"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

func writePromoted(out io.Writer, plan TemplatePlan) error {
	if len(plan.Promoted) == 0 {
		return nil
	}
	paths := make([]string, 0, len(plan.Promoted))
	for _, requirement := range plan.Promoted {
		paths = append(paths, requirement.Path)
	}
	message := "go.mod marca como indiretos módulos que o código importa diretamente: %s. " +
		"O registro herdado já os lista como dependências diretas; rode go mod tidy para que go.mod diga o mesmo.\n"
	if plan.Language != "" && plan.Language != model.DefaultLanguage {
		message = "go.mod marks as indirect modules the code imports directly: %s. " +
			"The inherited baseline already lists them as direct dependencies; run go mod tidy so go.mod says the same.\n"
	}
	if _, err := fmt.Fprintf(out, message, strings.Join(paths, ", ")); err != nil {
		return fmt.Errorf("bootstrap: write promoted imports: %w", err)
	}
	return nil
}
