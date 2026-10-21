package report_test

// policyLintSentences spells the sentence of every code the lint half of
// the policy family emits, as TestHumanRendersPortugueseForEveryCode reads
// them.
func policyLintSentences() map[string]string {
	return map[string]string{
		"golangci-missing":            "configuração do golangci-lint ausente na raiz; gere o .golangci.yml com o golaw init",
		"golangci-linter-off":         "linter exigido pela política não está ativo; ative-o em linters.enable",
		"golangci-linter-disabled":    "linter exigido pela política está em linters.disable; retire-o da lista",
		"golangci-hollowed":           "regra de exclusão esvazia um linter exigido; remova a regra",
		"golangci-exclusion-unlisted": "exclusão de um linter exigido fora das exclusões permitidas do golaw.yml; remova a exclusão",
		"golangci-shadow-off":         "o govet não roda a análise shadow; ative enable-all ou acrescente shadow",
	}
}
