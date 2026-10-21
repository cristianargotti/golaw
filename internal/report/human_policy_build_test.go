package report_test

// policyBuildSentences spells the sentence of every code the build half of
// the policy family emits, as TestHumanRendersPortugueseForEveryCode reads
// them.
func policyBuildSentences() map[string]string {
	return map[string]string{
		"make-file-missing":       "Makefile ausente; adicione o arquivo com os alvos exigidos",
		"make-target-missing":     "alvo obrigatório ausente; adicione o alvo indicado ao Makefile",
		"make-include-missing":    "arquivo incluído ausente; restaure o arquivo indicado ou corrija o include",
		"make-recipe-overridden":  "receita de alvo obrigatório definida duas vezes; renomeie um dos alvos",
		"ci-target-missing":       "verificação ausente do CI; inclua o alvo indicado em ci ou em uma execução sob demanda comprovada",
		"hook-command-not-binary": "verificação pesada em um gancho local; deixe-a no make ci e chame só o binário golaw no gancho",
		"generate-check-missing":  "alvo generate-check ausente; adicione o alvo ao Makefile ou a um arquivo incluído",
	}
}
