package bootstrap

import (
	"fmt"
	"io"
)

func writeOwnership(out io.Writer, plan TemplatePlan) error {
	protected := "Destino protegido por CODEOWNERS, preservado: %s.\n"
	irregular := "Destino com link simbólico ou caminho irregular, preservado: %s.\n"
	unsupported := "Limite de leitura, sem atribuição de propriedade: %s\n"
	if plan.Language != "" && plan.Language != "pt-BR" {
		protected = "Destination protected by CODEOWNERS, preserved: %s.\n"
		irregular = "Destination with a symbolic link or an irregular path, preserved: %s.\n"
		unsupported = "Read limit, no ownership assigned: %s\n"
	}
	for _, group := range []struct {
		format string
		lines  []string
	}{
		{protected, plan.Ownership.Protected}, {irregular, plan.Irregular}, {unsupported, plan.Ownership.Unsupported},
	} {
		for _, line := range group.lines {
			if _, err := fmt.Fprintf(out, group.format, line); err != nil {
				return fmt.Errorf("bootstrap: write ownership notice: %w", err)
			}
		}
	}
	return nil
}
