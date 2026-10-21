package bootstrap

import (
	"fmt"
	"io"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

func writeCollisions(out io.Writer, plan TemplatePlan) error {
	if len(plan.Collisions) == 0 {
		return nil
	}
	targets := make([]string, 0, len(plan.Collisions))
	for _, collision := range plan.Collisions {
		targets = append(targets, fmt.Sprintf("%s (Makefile:%d)", collision.Target, collision.Line))
	}
	prefix := "Alvos que o Makefile e golaw.mk definem, e com include golaw.mk o make roda só a receita de golaw.mk: "
	remedy := ". Renomeie ou remova cada alvo no Makefile antes de aceitar a proposta do include."
	if plan.Language != "" && plan.Language != model.DefaultLanguage {
		prefix = "Targets the Makefile and golaw.mk both define, and with include golaw.mk make runs only the recipe of golaw.mk: "
		remedy = ". Rename or remove each target in the Makefile before you accept the include proposal."
	}
	if _, err := fmt.Fprintln(out, prefix+strings.Join(targets, ", ")+remedy); err != nil {
		return fmt.Errorf("bootstrap: write collisions: %w", err)
	}
	return nil
}
