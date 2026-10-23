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
	}
}
