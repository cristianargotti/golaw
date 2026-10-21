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
		{code: "ci-target-missing", text: "verificação ausente do CI; inclua o alvo indicado em ci ou em uma execução sob demanda comprovada"},
		{code: "hook-command-not-binary", text: "verificação pesada em um gancho local; deixe-a no make ci e chame só o binário golaw no gancho"},
	}
}
