package hook

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/report"
)

// surfaceDenial is the decision of a hook whose first enforced finding is f, a
// note before it, with the reason the engine writes.
func surfaceDenial(f model.Finding) engine.Decision {
	note := model.Finding{Check: "surface.html-prefer-tag", Path: "views/a.html", Line: 1, Code: "prefer-tag", Message: "a note", Severity: model.SeverityNote}
	return engine.Decision{Mode: model.ModeHook, Verdict: engine.VerdictDeny, Findings: []model.Finding{note, f}, Reason: f.Check + ": " + f.Message + " (" + f.Path + ")"}
}

func TestSurfaceDenialsGiveTheReasonAndTheFix(t *testing.T) {
	t.Parallel()
	evalMessage := "runs text as code with new Function (Sonar S1523): export the code from a module and call it; writing it to a file and requiring it is the same execution."
	tests := []struct {
		name    string
		finding model.Finding
		reason  string
		human   string
	}{
		{
			"code-eval",
			model.Finding{Check: "surface.code-eval", Path: "test/view/host.test.cjs", Line: 25, Code: "code-eval", Message: evalMessage, Severity: model.SeverityDeny},
			"test/view/host.test.cjs:25 " + evalMessage,
			"A operação foi bloqueada pela verificação surface.code-eval no caminho test/view/host.test.cjs. Exporte o código de um módulo e chame-o diretamente; não execute texto como código.",
		},
		{
			"file-length without a line",
			model.Finding{Check: "surface.file-length", Path: "scripts/smoke.sh", Code: "file-length", Message: "has 312 lines, the limit is 300: split it by responsibility.", Severity: model.SeverityDeny},
			"scripts/smoke.sh has 312 lines, the limit is 300: split it by responsibility.",
			"A operação foi bloqueada pela verificação surface.file-length no caminho scripts/smoke.sh. Divida o arquivo por responsabilidade; o limite é 300 linhas.",
		},
		{
			"analyzer-widening",
			model.Finding{Check: "surface.analyzer-widening", Path: "sonar-project.properties", Code: "analyzer-widening", Message: "adds src/legacy/** to sonar.exclusions: an exclusion hides code from the pipeline's gate; fix the findings, or ask the person to change the scope.", Severity: model.SeverityDeny},
			"sonar-project.properties adds src/legacy/** to sonar.exclusions: an exclusion hides code from the pipeline's gate; fix the findings, or ask the person to change the scope.",
			"A operação foi bloqueada pela verificação surface.analyzer-widening no caminho sonar-project.properties. Uma exclusão esconde código do gate do pipeline; corrija os problemas ou peça a mudança a quem cuida do repositório.",
		},
		{
			"tag-and-digest",
			model.Finding{Check: "docker.tag-and-digest", Path: "Dockerfile", Line: 49, Code: "tag-and-digest", Message: "writes FROM with a tag and a digest (Sonar S8431): keep only the digest and put the tag in a comment on the line above.", Severity: model.SeverityDeny},
			"Dockerfile:49 writes FROM with a tag and a digest (Sonar S8431): keep only the digest and put the tag in a comment on the line above.",
			"A operação foi bloqueada pela verificação docker.tag-and-digest no caminho Dockerfile. Mantenha só o digest e deixe a tag em um comentário na linha de cima.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := decisionResult(model.Event{Shape: model.ShapeWrite}, surfaceDenial(tt.finding), model.DefaultLanguage)
			if result.Outcome != Deny || result.Reason != tt.reason || result.Human != tt.human || result.CannotJudge {
				t.Fatalf("result = %+v\nwant reason %q\nand human %q", result, tt.reason, tt.human)
			}
		})
	}
}

func TestSurfaceReasonsKeepTheOtherLanguagesAndFamilies(t *testing.T) {
	t.Parallel()
	f := model.Finding{Check: "surface.code-eval", Path: "a.js", Line: 2, Code: "code-eval", Message: "runs text as code with eval (Sonar S1523): x.", Severity: model.SeverityDeny}
	english := decisionResult(model.Event{Shape: model.ShapeWrite}, surfaceDenial(f), "en")
	if english.Reason != "a.js:2 runs text as code with eval (Sonar S1523): x." || english.Human != "The operation was blocked by check surface.code-eval at path a.js." {
		t.Errorf("english = %+v", english)
	}
	post := decisionResult(model.Event{Shape: model.ShapePost}, surfaceDenial(f), model.DefaultLanguage)
	if post.Outcome != Feedback || post.Human != "Corrija a verificação surface.code-eval no caminho a.js antes de continuar. Exporte o código de um módulo e chame-o diretamente; não execute texto como código." {
		t.Errorf("post = %+v", post)
	}
	g := model.Finding{Check: "goast.file-length", Path: "a.go", Code: "file-length", Message: "file has 301 lines, the limit is 300: split it by responsibility", Severity: model.SeverityDeny}
	decision := engine.Decision{Mode: model.ModeHook, Verdict: engine.VerdictDeny, Findings: []model.Finding{g}, Reason: "goast.file-length: file has 301 lines, the limit is 300: split it by responsibility (a.go)"}
	goResult := decisionResult(model.Event{Shape: model.ShapeWrite}, decision, model.DefaultLanguage)
	if goResult.Reason != decision.Reason || goResult.Human != report.Human(model.DefaultLanguage, decision) {
		t.Errorf("a go denial changed: %+v", goResult)
	}
}

func TestWithLimitKeepsAFixWithoutTheNumber(t *testing.T) {
	t.Parallel()
	fix := "Divida o arquivo por responsabilidade."
	if got := withLimit(fix, "is too long: split it by responsibility."); got != fix {
		t.Errorf("fix = %q, want it unchanged", got)
	}
}
