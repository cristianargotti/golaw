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
	}
}
