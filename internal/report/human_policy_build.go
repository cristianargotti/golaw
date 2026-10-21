package report

// policyBuildPhrases are the sentences of the codes the build half of the
// policy family emits (policy.makefile, policy.ci-target,
// policy.generated-fresh, policy.hook-tests), one per code, in the order
// its checks register.
func policyBuildPhrases() []sentence {
	return []sentence{
		{code: "make-file-missing", text: "Makefile ausente; adicione o arquivo com os alvos exigidos"},
		{code: "make-target-missing", text: "alvo obrigatório ausente; adicione o alvo indicado ao Makefile"},
		{code: "make-include-missing", text: "arquivo incluído ausente; restaure o arquivo indicado ou corrija o include"},
		{code: "make-recipe-overridden", text: "receita de alvo obrigatório definida duas vezes; renomeie um dos alvos"},
	}
}
