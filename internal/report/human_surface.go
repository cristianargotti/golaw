package report

// surfacePhrases are the sentences of the codes the native checks of the
// surface family emit, one per code; surface.file-length shares the
// file-length code of goast and so its sentence, because the table holds
// one sentence per code.
func surfacePhrases() []sentence {
	return []sentence{
		{code: "code-eval", text: "texto executado como código; exporte o código de um módulo e chame-o diretamente; não execute texto como código"},
		{code: "hardcoded-credential", text: "credencial literal no código; gere a credencial em tempo de execução ou leia-a do ambiente"},
		{code: "inline-script-size", text: "script embutido acima de 40 linhas; mova o script para um arquivo .js e incorpore-o ao servir a página"},
		{code: "prefer-tag", text: "papel ARIA no lugar do elemento nativo; use o elemento nativo, como output no lugar de role status"},
		{code: "python-function-length", text: "função Python acima do limite de linhas da política; extraia passos com nome próprio"},
		{code: "suppression", text: "supressão que esconde a linha do analisador; corrija o problema em vez de escondê-lo do analisador"},
		{code: "analyzer-widening", text: "escopo do analisador ampliado por um agente; uma exclusão esconde código do gate do pipeline; corrija os problemas ou peça a mudança a quem cuida do repositório"},
	}
}
