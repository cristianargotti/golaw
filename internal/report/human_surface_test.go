package report_test

// surfaceSentences spells the sentence of every code the surface family
// emits, as TestHumanRendersPortugueseForEveryCode reads them;
// surface.file-length reads the file-length sentence of goast.
func surfaceSentences() map[string]string {
	return map[string]string{
		"code-eval":              "texto executado como código; exporte o código de um módulo e chame-o diretamente; não execute texto como código",
		"hardcoded-credential":   "credencial literal no código; gere a credencial em tempo de execução ou leia-a do ambiente",
		"inline-script-size":     "script embutido acima de 40 linhas; mova o script para um arquivo .js e incorpore-o ao servir a página",
		"prefer-tag":             "papel ARIA no lugar do elemento nativo; use o elemento nativo, como output no lugar de role status",
		"python-function-length": "função Python acima do limite de linhas da política; extraia passos com nome próprio",
		"suppression":            "supressão que esconde a linha do analisador; corrija o problema em vez de escondê-lo do analisador",
		"analyzer-widening":      "escopo do analisador ampliado por um agente; uma exclusão esconde código do gate do pipeline; corrija os problemas ou peça a mudança a quem cuida do repositório",
	}
}
