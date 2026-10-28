package report

import (
	"fmt"
	"slices"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// Portuguese is the language of the human channel by default.
const Portuguese = "pt-BR"

// Human renders the sentence a person reads for a decision: Portuguese
// (pt-BR) through the phrase table, English for any other language. The
// agent reads decision.Reason, never this sentence, so the English detail
// of a finding or of an unmeasured check stays in the reason and the report.
func Human(language string, decision engine.Decision) string {
	if language != Portuguese {
		return english(decision)
	}
	for k := range decision.Findings {
		f := &decision.Findings[k]
		if f.Enforced() {
			return "golaw bloqueou a operação: " + phrase(*f) + "."
		}
	}
	if len(decision.Unmeasured) > 0 {
		return fmt.Sprintf("golaw não conseguiu medir a verificação %s.", decision.Unmeasured[0].Check)
	}
	return "golaw: nenhuma violação encontrada."
}

// english is the human channel for a language without a phrase table.
func english(decision engine.Decision) string {
	if decision.Reason == "" {
		return "golaw: no violation found."
	}
	return "golaw blocked the operation: " + decision.Reason + "."
}

// phrase quotes neither the finding's English message nor a command or a
// file content, so the sentence stays in Portuguese and carries no secret
// value; a code outside the table (a future family) still names its check
// and its place.
func phrase(f model.Finding) string {
	text, ok := lookup(f.Code)
	if !ok {
		text = "violação de regra"
	}
	where := ""
	switch {
	case f.Path != "" && f.Line > 0:
		where = fmt.Sprintf(", arquivo %s, linha %d", f.Path, f.Line)
	case f.Path != "":
		where = ", arquivo " + f.Path
	}
	return fmt.Sprintf("%s (verificação %s%s)", text, f.Check, where)
}

// sentence is one row of the human channel table.
type sentence struct {
	code string
	text string
}

func lookup(code string) (string, bool) {
	for _, s := range phrases() {
		if s.code == code {
			return s.text, true
		}
	}
	return "", false
}

// phrases is the message table of the human channel: one Portuguese
// sentence per finding code of the kit, in catalog order; each half of the
// policy family keeps its rows in a file of its own.
func phrases() []sentence {
	return slices.Concat(filePhrases(), goastPhrases(), shellPhrases(), commitPhrases(),
		policyLintPhrases(), policyBuildPhrases(), policyDepsPhrases(), policyProfilePhrases())
}

// filePhrases are the sentences of the text and paths families.
func filePhrases() []sentence {
	return []sentence{
		{code: "em-dash", text: "travessão no texto; use vírgula, dois pontos ou hífen"},
		{code: "en-dash", text: "meia-risca no texto; use vírgula, dois pontos ou hífen"},
		{code: "trailer", text: "trailer de commit no texto ou na mensagem; o kit não aceita trailers"},
		{code: "trace-term", text: "palavra de rastro de orquestração no texto; a árvore pública não carrega rastros de sessão"},
		{code: "home-path", text: "caminho absoluto de home no texto; use um caminho relativo ao repositório"},
		{code: "tool-term", text: "nome de ferramenta fora dos caminhos permitidos; a árvore pública não nomeia ferramentas"},
		{code: "comment-language", text: "comentário de código fora do inglês; comentários são em inglês e só explicam o porquê"},
		{code: "todo-marker", text: "marcador de tarefa no código; termine o trabalho ou registre a decisão"},
		{code: "bare-nolint", text: "supressão de lint sem o linter ou sem justificativa"},
		{code: "nolint-nilnil", text: "retorno nil, nil escondido atrás de uma supressão; responda um valor ou um sentinela tipado"},
		{code: "debug-print", text: "impressão na saída padrão em código de aplicação; escreva em um io.Writer ou no logger"},
		{code: "binary-document", text: "imagem ou documento fora de public/, manual/, fixtures/ ou testdata/"},
		{code: "secret-file", text: "arquivo com nome de material secreto na árvore; segredos nunca vivem no repositório"},
		{code: "generated-ignored", text: "regra de ignore esconde uma árvore gerada; código gerado é versionado"},
		{code: "secret-material", text: "leitura de material secreto; segredos nunca entram no contexto"},
	}
}

// goastPhrases are the sentences of the goast family, one per code its
// checks emit.
func goastPhrases() []sentence {
	return []sentence{
		{code: "file-length", text: "arquivo acima do limite de linhas da política; divida o arquivo por responsabilidade"},
		{code: "function-length", text: "função acima do limite de linhas da política; extraia passos com nome próprio"},
		{code: "function-statements", text: "função acima do limite de instruções da política; extraia passos com nome próprio"},
		{code: "env-read", text: "leitura de variável de ambiente fora da raiz de composição; receba o valor pela configuração"},
		{code: "bare-context", text: "contexto criado fora da raiz de composição; receba o contexto de quem chama"},
		{code: "wall-clock", text: "relógio do processo no domínio ou na aplicação; receba o tempo por parâmetro ou pela porta Clock"},
		{code: "local-clock", text: "relógio entregue aos contextos fora de UTC; entregue um relógio em UTC"},
		{code: "sql-from-strings", text: "consulta SQL montada a partir de strings; use as consultas geradas"},
		{code: "raw-error-response", text: "resposta de erro escrita fora do pacote de erros; responda pela porta única de erros"},
		{code: "error-text-as-data", text: "texto de um erro guardado como dado; guarde o erro, nunca o seu texto"},
		{code: "init-if-err-nil", text: "if com inicialização que testa err == nil; trate o erro primeiro"},
		{code: "sentinel-decl", text: "sentinela declarado fora de errors.go; declare os modos de falha do pacote em errors.go"},
		{code: "secret-field", text: "credencial em um campo string simples; use o tipo que oculta o valor"},
		{code: "goleak", text: "pacote que inicia goroutines sem teste de vazamento; adicione a verificação de goroutines"},
		{code: "adapter-package-size", text: "raiz do pacote adaptador com arquivos demais; mova cada adaptador para o seu subpacote"},
		{code: "empty-body", text: "função com corpo vazio e sem comentário; implemente ou explique por que está vazia"},
		{code: "literal-repeat", text: "literal repetido no arquivo; extraia uma constante"},
		{code: "identical-functions", text: "duas funções idênticas no mesmo arquivo; reutilize uma delas"},
	}
}

// shellPhrases are the sentences of the shell family.
func shellPhrases() []sentence {
	return []sentence{
		{code: "secret-token", text: "o comando nomeia material secreto; segredos nunca entram no contexto"},
		{code: "no-verify", text: "o comando pula os hooks do git; os hooks são a regra"},
		{code: "body-width", text: "linha do corpo do commit acima de 100 caracteres; quebre o corpo"},
		{code: "commit-shape", text: "commit que não é feat carrega mais de um tema; divida por tema ou nomeie como feat"},
		{code: "credential-print", text: "o comando imprime uma credencial; segredos nunca entram no contexto"},
		{code: "push", text: "git push negado pela política; a pessoa faz o push"},
		{code: "pr", text: "criação de pull request ou de repositório negada pela política; a pessoa abre"},
	}
}

// commitPhrases are the sentences of the codes only the commits family
// emits; its dash, trailer, width and shape findings share the codes, and
// so the sentences, of the text and shell families.
func commitPhrases() []sentence {
	return []sentence{
		{code: "conventional-subject", text: "assunto do commit fora do padrão convencional; escreva tipo(escopo): descrição, com a descrição começando em letra minúscula"},
	}
}
