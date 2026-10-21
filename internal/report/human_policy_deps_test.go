package report_test

// policyDepsSentences spells the sentence of every code the dependency
// half of the policy family emits, as TestHumanRendersPortugueseForEveryCode
// reads them.
func policyDepsSentences() map[string]string {
	return map[string]string{
		"dependency-record-missing": "dependência direta sem registro de decisão; registre o módulo no cabeçalho do registro",
		"dependabot-cooldown-short": "intervalo de atualização abaixo da política; use sete dias e trinta para versões principais",
		"image-digest-missing":      "imagem base sem digest completo; fixe a referência com sha256",
		"openapi-policy":            "contrato de API sem asserções exigidas; use double e exemplos de resposta validados",
		"squawk-policy":             "configuração de migrações enfraquecida; restaure a versão e as regras de transação",
	}
}
