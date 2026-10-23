package report_test

// policyProfileSentences spells the sentence of every code the checks the
// profile data turns on emit, as TestHumanRendersPortugueseForEveryCode
// reads them.
func policyProfileSentences() map[string]string {
	return map[string]string{
		"image-stage-order":   "estágios do Dockerfile fora da ordem do perfil; reordene os estágios como o perfil define",
		"image-stage-missing": "estágio exigido pelo perfil ausente do Dockerfile; adicione o estágio que o perfil nomeia",
		"image-final-image":   "último estágio do Dockerfile fora da imagem final do perfil; parta da imagem que o perfil define",
	}
}
