package bootstrap

import (
	"fmt"
	"io"
)

// preservedTexts are the line formats for a destination init leaves as it
// is, each taking the path or the record it names.
type preservedTexts struct {
	protected, irregular, held, unsupported string
}

func preservedFormats(language string) preservedTexts {
	if language != "" && language != "pt-BR" {
		return preservedTexts{
			protected: "Destination protected by CODEOWNERS, preserved: %s.\n",
			irregular: "Destination with a symbolic link or an irregular path, preserved: %s.\n",
			held: "Template without a record, CODEOWNERS protects golaw.yml: %s; " +
				"an owner of golaw.yml records this hash under managed and commits the file.\n",
			unsupported: "Read limit, no ownership assigned: %s\n",
		}
	}
	return preservedTexts{
		protected: "Destino protegido por CODEOWNERS, preservado: %s.\n",
		irregular: "Destino com link simbólico ou caminho irregular, preservado: %s.\n",
		held: "Template sem registro, CODEOWNERS protege golaw.yml: %s; " +
			"um responsável por golaw.yml registra esse hash em managed e faz o commit do arquivo.\n",
		unsupported: "Limite de leitura, sem atribuição de propriedade: %s\n",
	}
}

func writeOwnership(out io.Writer, plan TemplatePlan) error {
	texts := preservedFormats(plan.Language)
	records := make([]string, 0, len(plan.Held))
	for _, record := range plan.Held {
		records = append(records, record.Path+" sha256="+record.SHA256)
	}
	for _, group := range []struct {
		format string
		lines  []string
	}{
		{texts.protected, plan.Ownership.Protected},
		{texts.irregular, plan.Irregular},
		{texts.held, records},
		{texts.unsupported, plan.Ownership.Unsupported},
	} {
		for _, line := range group.lines {
			if _, err := fmt.Fprintf(out, group.format, line); err != nil {
				return fmt.Errorf("bootstrap: write ownership notice: %w", err)
			}
		}
	}
	return nil
}
