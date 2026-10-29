package report_test

// surfaceSentences spells the sentence of every code the surface family
// emits, as TestHumanRendersPortugueseForEveryCode reads them;
// surface.file-length reads the file-length sentence of goast.
func surfaceSentences() map[string]string {
	return map[string]string{
		"code-eval": "texto executado como código; exporte o código de um módulo e chame-o diretamente; não execute texto como código",
	}
}
