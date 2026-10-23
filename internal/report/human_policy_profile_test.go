package report_test

// policyProfileSentences spells the sentence of every code the checks the
// profile data turns on emit, as TestHumanRendersPortugueseForEveryCode
// reads them.
func policyProfileSentences() map[string]string {
	return map[string]string{
		"image-stage-order":         "estágios do Dockerfile fora da ordem do perfil; reordene os estágios como o perfil define",
		"image-stage-missing":       "estágio exigido pelo perfil ausente do Dockerfile; adicione o estágio que o perfil nomeia",
		"image-final-image":         "último estágio do Dockerfile fora da imagem final do perfil; parta da imagem que o perfil define",
		"image-install-not-allowed": "pacote fora da lista permitida do estágio no Dockerfile; instale só os pacotes que o perfil permite",
		"image-install-unpinned":    "pacote instalado sem versão fixa no Dockerfile; escreva pacote=versão",
		"image-inline-ignore":       "comentário de supressão proibido pelo perfil no Dockerfile; corrija a regra em vez de suprimi-la",
		"image-registry":            "imagem base fora do registro que o perfil define; use uma imagem desse registro",
		"profile-target-missing":    "alvo exigido pelo perfil fora do alvo ci; adicione o alvo aos pré-requisitos de ci",
		"values-assertion":          "valor de configuração diferente do que o perfil exige; ajuste o tipo e o valor da chave",
		"values-key-missing":        "chave exigida pelo perfil ausente do arquivo de valores; adicione a chave",
		"values-file-missing":       "arquivo de valores exigido pelo perfil ausente; crie o arquivo com as chaves exigidas",
		"profile-forbidden-path":    "caminho proibido pelo perfil; remova o arquivo ou mantenha-o fora do repositório",
	}
}
