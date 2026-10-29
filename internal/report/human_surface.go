package report

// surfacePhrases are the sentences of the codes the native checks of the
// surface family emit, one per code; surface.file-length shares the
// file-length code of goast and so its sentence, because the table holds
// one sentence per code.
func surfacePhrases() []sentence {
	return []sentence{
		{code: "code-eval", text: "texto executado como código; exporte o código de um módulo e chame-o diretamente; não execute texto como código"},
		{code: "hardcoded-credential", text: "credencial literal no código; gere a credencial em tempo de execução ou leia-a do ambiente"},
		{code: "python-function-length", text: "função Python acima do limite de linhas da política; extraia passos com nome próprio"},
	}
}
