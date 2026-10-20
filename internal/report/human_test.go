package report_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/report"
)

func TestHumanRendersPortugueseAndFallsBackToEnglish(t *testing.T) {
	t.Parallel()
	_, denied := run(t)
	cases := map[string]string{
		"pt-BR": "golaw bloqueou a operação: travessão no texto; use vírgula, dois pontos ou hífen (verificação text.dashes, arquivo docs/a.md, linha 3).",
		"en":    "golaw blocked the operation: text.dashes: em dash in text (docs/a.md:3).",
	}
	for language, want := range cases {
		if got := report.Human(language, denied); got != want {
			t.Errorf("%s: %q, want %q", language, got, want)
		}
	}
	clean := engine.Decision{Mode: model.ModeHook, Verdict: engine.VerdictClean}
	if got := report.Human(report.Portuguese, clean); got != "golaw: nenhuma violação encontrada." {
		t.Errorf("clean = %q", got)
	}
	if got := report.Human("en", clean); got != "golaw: no violation found." {
		t.Errorf("clean in English = %q", got)
	}
}

func TestHumanNamesUnknownCodesAndUnmeasuredChecks(t *testing.T) {
	t.Parallel()
	unknown := engine.Decision{Mode: model.ModeHook, Verdict: engine.VerdictDeny, Findings: []model.Finding{
		{Check: "future.rule", Path: "internal/a.go", Code: "future-code", Message: "a code no family emits yet", Severity: model.SeverityFail},
	}}
	if got := report.Human(report.Portuguese, unknown); got != "golaw bloqueou a operação: violação de regra (verificação future.rule, arquivo internal/a.go)." {
		t.Errorf("unknown code = %q", got)
	}
	notes := engine.Decision{
		Mode: model.ModeHook, Verdict: engine.VerdictDeny,
		Findings:   []model.Finding{{Check: "shell.push", Code: "push", Message: "git push observed", Severity: model.SeverityNote}},
		Unmeasured: []engine.Unmeasured{{Check: "text.todo", Reason: "read a.go: path unreadable"}},
	}
	if got := report.Human(report.Portuguese, notes); got != "golaw não conseguiu medir a verificação text.todo." {
		t.Errorf("unmeasured = %q", got)
	}
	event := engine.Decision{Verdict: engine.VerdictDeny, Findings: []model.Finding{{Check: "shell.no-verify", Code: "no-verify", Message: "x", Severity: model.SeverityDeny}}}
	if got := report.Human(report.Portuguese, event); got != "golaw bloqueou a operação: o comando pula os hooks do git; os hooks são a regra (verificação shell.no-verify)." {
		t.Errorf("event finding = %q", got)
	}
}

// TestHumanRendersPortugueseForEveryCode spells the sentence of every
// finding code the checks emit, so a row deleted from the table or a
// sentence changed by mistake turns red instead of passing as the generic
// sentence.
func TestHumanRendersPortugueseForEveryCode(t *testing.T) {
	t.Parallel()
	for _, family := range []map[string]string{fileSentences(), goastSentences(), shellSentences(), commitSentences()} {
		for code, want := range family {
			decision := engine.Decision{Mode: model.ModeHook, Verdict: engine.VerdictDeny, Findings: []model.Finding{
				{Check: "family.sample", Path: "internal/a.go", Code: code, Message: "english message", Severity: model.SeverityDeny},
			}}
			got := report.Human(report.Portuguese, decision)
			if got != "golaw bloqueou a operação: "+want+" (verificação family.sample, arquivo internal/a.go)." || strings.Contains(got, "english message") {
				t.Errorf("%s = %q", code, got)
			}
		}
	}
}

func fileSentences() map[string]string {
	return map[string]string{
		"em-dash":           "travessão no texto; use vírgula, dois pontos ou hífen",
		"en-dash":           "meia-risca no texto; use vírgula, dois pontos ou hífen",
		"trailer":           "trailer de commit no texto ou na mensagem; o kit não aceita trailers",
		"trace-term":        "palavra de rastro de orquestração no texto; a árvore pública não carrega rastros de sessão",
		"home-path":         "caminho absoluto de home no texto; use um caminho relativo ao repositório",
		"tool-term":         "nome de ferramenta fora dos caminhos permitidos; a árvore pública não nomeia ferramentas",
		"comment-language":  "comentário de código fora do inglês; comentários são em inglês e só explicam o porquê",
		"todo-marker":       "marcador de tarefa no código; termine o trabalho ou registre a decisão",
		"bare-nolint":       "supressão de lint sem o linter ou sem justificativa",
		"nolint-nilnil":     "retorno nil, nil escondido atrás de uma supressão; responda um valor ou um sentinela tipado",
		"debug-print":       "impressão na saída padrão em código de aplicação; escreva em um io.Writer ou no logger",
		"binary-document":   "imagem ou documento fora de public/, manual/, fixtures/ ou testdata/",
		"secret-file":       "arquivo com nome de material secreto na árvore; segredos nunca vivem no repositório",
		"generated-ignored": "regra de ignore esconde uma árvore gerada; código gerado é versionado",
		"secret-material":   "leitura de material secreto; segredos nunca entram no contexto",
	}
}

func goastSentences() map[string]string {
	return map[string]string{
		"file-length":          "arquivo acima do limite de linhas da política; divida o arquivo por responsabilidade",
		"function-length":      "função acima do limite de linhas da política; extraia passos com nome próprio",
		"function-statements":  "função acima do limite de instruções da política; extraia passos com nome próprio",
		"env-read":             "leitura de variável de ambiente fora da raiz de composição; receba o valor pela configuração",
		"bare-context":         "contexto criado fora da raiz de composição; receba o contexto de quem chama",
		"wall-clock":           "relógio do processo no domínio ou na aplicação; receba o tempo por parâmetro ou pela porta Clock",
		"local-clock":          "relógio entregue aos contextos fora de UTC; entregue um relógio em UTC",
		"sql-from-strings":     "consulta SQL montada a partir de strings; use as consultas geradas",
		"raw-error-response":   "resposta de erro escrita fora do pacote de erros; responda pela porta única de erros",
		"error-text-as-data":   "texto de um erro guardado como dado; guarde o erro, nunca o seu texto",
		"init-if-err-nil":      "if com inicialização que testa err == nil; trate o erro primeiro",
		"sentinel-decl":        "sentinela declarado fora de errors.go; declare os modos de falha do pacote em errors.go",
		"secret-field":         "credencial em um campo string simples; use o tipo que oculta o valor",
		"goleak":               "pacote que inicia goroutines sem teste de vazamento; adicione a verificação de goroutines",
		"adapter-package-size": "raiz do pacote adaptador com arquivos demais; mova cada adaptador para o seu subpacote",
		"empty-body":           "função com corpo vazio e sem comentário; implemente ou explique por que está vazia",
		"literal-repeat":       "literal repetido no arquivo; extraia uma constante",
		"identical-functions":  "duas funções idênticas no mesmo arquivo; reutilize uma delas",
	}
}

func shellSentences() map[string]string {
	return map[string]string{
		"secret-token":     "o comando nomeia material secreto; segredos nunca entram no contexto",
		"no-verify":        "o comando pula os hooks do git; os hooks são a regra",
		"body-width":       "linha do corpo do commit acima de 100 caracteres; quebre o corpo",
		"commit-shape":     "commit que não é feat carrega mais de um tema; divida por tema ou nomeie como feat",
		"credential-print": "o comando imprime uma credencial; segredos nunca entram no contexto",
		"push":             "git push negado pela política; a pessoa faz o push",
		"pr":               "criação de pull request ou de repositório negada pela política; a pessoa abre",
	}
}

func commitSentences() map[string]string {
	return map[string]string{
		"conventional-subject": "assunto do commit fora do padrão convencional; escreva tipo(escopo): descrição, com a descrição começando em letra minúscula",
	}
}
