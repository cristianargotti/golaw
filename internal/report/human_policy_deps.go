package report

// policyDepsPhrases are the sentences of the codes the dependency half of
// the policy family emits (policy.dependency-records,
// policy.dependabot-cooldown, policy.deps-age-config, policy.image-digest,
// policy.codeowners-foreign, policy.openapi-assertions,
// policy.squawk-config), one per code, in the order its checks register.
func policyDepsPhrases() []sentence {
	return []sentence{
		{"dependency-record-missing", "dependência direta sem registro de decisão; registre o módulo no cabeçalho do registro"},
		{"dependabot-cooldown-short", "intervalo de atualização abaixo da política; use sete dias e trinta para versões principais"},
		{"deps-age-minimum", "verificação de idade ausente ou abaixo de sete dias; corrija o alvo deps-age"},
		{"image-digest-missing", "imagem base sem digest completo; fixe a referência com sha256"},
		{"foreign-owned-change", "alteração em caminho de outro responsável; encaminhe a mudança à equipe proprietária"},
		{"openapi-policy", "contrato de API sem asserções exigidas; use double e exemplos de resposta validados"},
		{"squawk-policy", "configuração de migrações enfraquecida; restaure a versão e as regras de transação"},
	}
}
