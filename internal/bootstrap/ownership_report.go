package bootstrap

import (
	"fmt"
	"io"
)

func writeOwnership(out io.Writer, ownership Ownership, language string) error {
	protected := "Destino protegido por CODEOWNERS, preservado: %s.\n"
	unsupported := "Limite de leitura, sem atribuição de propriedade: %s\n"
	if language != "" && language != "pt-BR" {
		protected = "Destination protected by CODEOWNERS, preserved: %s.\n"
		unsupported = "Read limit, no ownership assigned: %s\n"
	}
	for _, group := range []struct {
		format string
		lines  []string
	}{{protected, ownership.Protected}, {unsupported, ownership.Unsupported}} {
		for _, line := range group.lines {
			if _, err := fmt.Fprintf(out, group.format, line); err != nil {
				return fmt.Errorf("bootstrap: write ownership notice: %w", err)
			}
		}
	}
	return nil
}
