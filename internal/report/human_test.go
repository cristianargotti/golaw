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
		{Check: "commit.conventional", Path: "internal/a.go", Code: "conventional-subject", Message: "subject is not conventional", Severity: model.SeverityFail},
	}}
	if got := report.Human(report.Portuguese, unknown); got != "golaw bloqueou a operação: violação de regra (verificação commit.conventional, arquivo internal/a.go)." {
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

// goastCodes are the finding codes the goast family emits, the funlen
// statement count beside the line count.
func goastCodes() []string {
	return []string{
		"file-length", "function-length", "function-statements", "env-read",
		"bare-context", "wall-clock", "local-clock", "sql-from-strings",
		"raw-error-response", "error-text-as-data", "init-if-err-nil", "sentinel-decl",
		"secret-field", "goleak", "adapter-package-size", "empty-body",
		"literal-repeat", "identical-functions",
	}
}

func TestHumanRendersPortugueseForEveryGoastCode(t *testing.T) {
	t.Parallel()
	for _, code := range goastCodes() {
		decision := engine.Decision{Mode: model.ModeHook, Verdict: engine.VerdictDeny, Findings: []model.Finding{
			{Check: "goast.sample", Path: "internal/a.go", Code: code, Message: "english message", Severity: model.SeverityDeny},
		}}
		got := report.Human(report.Portuguese, decision)
		if strings.Contains(got, "english message") || !strings.HasSuffix(got, "(verificação goast.sample, arquivo internal/a.go).") {
			t.Errorf("%s = %q", code, got)
		}
	}
}
