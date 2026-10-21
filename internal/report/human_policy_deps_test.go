package report_test

// policyDepsSentences spells the sentence of every code the dependency
// half of the policy family emits, as TestHumanRendersPortugueseForEveryCode
// reads them.
func policyDepsSentences() map[string]string {
	return map[string]string{
		"dependency-record-missing": "dependência direta sem registro de decisão; registre o módulo no cabeçalho do registro",
		"dependabot-cooldown-short": "intervalo de atualização abaixo da política; use sete dias e trinta para versões principais",
	}
}
