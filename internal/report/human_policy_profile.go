package report

// policyProfilePhrases are the sentences of the codes the checks the
// profile data turns on emit (policy.image-stages, policy.image-installs,
// policy.image-registry, policy.profile-targets, policy.values-assertions
// and paths.profile-forbidden), one per code, in the order their checks
// register.
func policyProfilePhrases() []sentence {
	return []sentence{
		{"image-stage-order", "estágios do Dockerfile fora da ordem do perfil; reordene os estágios como o perfil define"},
		{"image-stage-missing", "estágio exigido pelo perfil ausente do Dockerfile; adicione o estágio que o perfil nomeia"},
		{"image-final-image", "último estágio do Dockerfile fora da imagem final do perfil; parta da imagem que o perfil define"},
		{"image-install-not-allowed", "pacote fora da lista permitida do estágio no Dockerfile; instale só os pacotes que o perfil permite"},
		{"image-install-unpinned", "pacote instalado sem versão fixa no Dockerfile; escreva pacote=versão"},
		{"image-inline-ignore", "comentário de supressão proibido pelo perfil no Dockerfile; corrija a regra em vez de suprimi-la"},
		{"image-registry", "imagem base fora do registro que o perfil define; use uma imagem desse registro"},
	}
}
