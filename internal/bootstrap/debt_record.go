package bootstrap

import (
	"fmt"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// debtFrontMatter marks the record as a decision record without modules,
// so the dependency records rule reads nothing from it.
const debtFrontMatter = "---\nrecord: 1\ntitle: inherited debt\nstatus: inherited\n---\n"

// debtTexts are the words of the record in one language.
type debtTexts struct {
	title, intro, exceptions, exemptions, waivers string
}

// debtWords is Portuguese for pt-BR and English for any other language,
// the rule of the inherited baseline the record sits beside (decision 6).
func debtWords(language string) debtTexts {
	if language == model.DefaultLanguage {
		return debtTexts{
			title: "# Dívida herdada",
			intro: "Em %s, golaw init --apply --seed-debt registrou o que o repositório já descumpria na adoção. " +
				"Cada entrada abaixo está em golaw.yml com este registro e um prazo. A lista só diminui: uma entrada sai " +
				"de golaw.yml quando a correção chega, e o golaw recusa uma entrada nova ou um prazo maior.",
			exceptions: "| Verificação | Caminho | Até |",
			exemptions: "| Isenção de cobertura |",
			waivers:    "| Aviso de vulnerabilidade | Até |",
		}
	}
	return debtTexts{
		title: "# Inherited debt",
		intro: "On %s, golaw init --apply --seed-debt recorded what the repository already broke at adoption. " +
			"Every entry below is in golaw.yml with this record and an expiry. The list only shrinks: an entry leaves " +
			"golaw.yml when its fix lands, and golaw refuses a new entry or a later expiry.",
		exceptions: "| Check | Path | Until |",
		exemptions: "| Coverage exemption |",
		waivers:    "| Vulnerability waiver | Until |",
	}
}

// DebtRecord renders <decisions>/0002-inherited-debt.md: the day, every
// entry with its check, path or advisory id and expiry, and the rule that
// the list only shrinks, in the language of the policy. A path is written
// as the quoted ASCII scalar golaw.yml holds, and an empty table is left
// out.
func DebtRecord(d Debt, day, language string) []byte {
	words := debtWords(language)
	lines := []string{debtFrontMatter, words.title, "", fmt.Sprintf(words.intro, day)}
	exceptions := make([]string, 0, len(d.Exceptions))
	for _, e := range d.Exceptions {
		exceptions = append(exceptions, row(e.Check, quoted(e.Path), e.Until))
	}
	exemptions := make([]string, 0, len(d.Exemptions))
	for _, path := range d.Exemptions {
		exemptions = append(exemptions, row(quoted(path)))
	}
	waivers := make([]string, 0, len(d.Waivers))
	for _, w := range d.Waivers {
		waivers = append(waivers, row(w.ID, w.Until))
	}
	lines = table(lines, words.exceptions, exceptions)
	lines = table(lines, words.exemptions, exemptions)
	lines = table(lines, words.waivers, waivers)
	return []byte(strings.Join(lines, "\n") + "\n")
}

// table appends a Markdown table after a blank line, nothing without rows.
func table(lines []string, header string, rows []string) []string {
	if len(rows) == 0 {
		return lines
	}
	separator := "|" + strings.Repeat(" --- |", strings.Count(header, "|")-1)
	return append(append(lines, "", header, separator), rows...)
}

// row joins the cells of one table row, each with the one character that
// would split a Markdown cell escaped.
func row(cells ...string) string {
	escaped := make([]string, 0, len(cells))
	for _, c := range cells {
		escaped = append(escaped, strings.ReplaceAll(c, "|", `\|`))
	}
	return "| " + strings.Join(escaped, " | ") + " |"
}
