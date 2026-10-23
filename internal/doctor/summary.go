package doctor

import (
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// summary is the sentence the person reads under the lines of an agent in
// the text output, never in the JSON: the state line said in the
// configured language, Portuguese through the phrase table and English for
// any other language, with the fix it needs.
func summary(language string, f agentFacts, state Line) string {
	portuguese := language == "" || language == model.DefaultLanguage
	phrases := englishSummaries()
	if portuguese {
		phrases = portugueseSummaries()
	}
	key := state.Code
	switch {
	case key == codeNotInstalled && state.State == stateUnknown:
		key = summaryAbsent
	case key == codeInstalledUntrusted && f.Trust.Feature == featureOff:
		key = summaryFeatureOff
	}
	text, found := phrases[key]
	if !found {
		return ""
	}
	text = strings.NewReplacer("{agent}", f.Agent, "{command}", f.installCommand()).Replace(text)
	if f.Agent == agentCursor {
		text += " " + previewSentence(portuguese)
	}
	return text
}

// summaryAbsent is the phrase of an agent with no receipt at the scope,
// which is no finding: the person may use another agent. summaryFeatureOff
// is the phrase of a Codex whose hooks feature is off, where no /hooks
// approval helps.
const (
	summaryAbsent     = "summary-absent"
	summaryFeatureOff = "summary-feature-off"
)

// portugueseSummaries is the phrase table of the human channel, one
// sentence per code of the state line.
func portugueseSummaries() map[string]string {
	return map[string]string{
		codeProtecting:         "golaw: o hook de {agent} está instalado, é confiável e já rodou neste repositório.",
		codeInstalledUntrusted: "golaw: o hook de {agent} está instalado, mas o agente não vai executá-lo; aprove-o no agente (no Codex, em /hooks) e rode golaw doctor de novo.",
		codeTrustedInactive:    "golaw: o hook de {agent} está instalado e é confiável, mas ainda não rodou neste repositório; use o agente aqui e rode golaw doctor de novo.",
		codeNotInstalled:       "golaw: a instalação de {agent} está ausente ou foi alterada; rode {command}.",
		codeTrustUnknown:       "golaw: o hook de {agent} está instalado; se o agente o executa depende de uma confiança que o doctor não consegue ler (veja docs/agent-contracts.md).",
		codePreviewActive:      "golaw: o hook de {agent} está instalado e as proteções de shell e de leitura rodaram; as escritas são capturadas, ainda não bloqueadas (veja docs/agent-contracts.md).",
		summaryAbsent:          "golaw: {agent} não está instalado neste escopo; para usá-lo, rode {command}.",
		summaryFeatureOff:      "golaw: o hook de {agent} está instalado, mas o recurso de hooks está desligado no config.toml do Codex; ative features.hooks e rode golaw doctor de novo.",
	}
}

// englishSummaries is the human channel for a language without a phrase
// table.
func englishSummaries() map[string]string {
	return map[string]string{
		codeProtecting:         "golaw: the {agent} hook is installed, trusted and has run in this repository.",
		codeInstalledUntrusted: "golaw: the {agent} hook is installed, but the agent will not run it; approve it in the agent (in Codex, with /hooks) and run golaw doctor again.",
		codeTrustedInactive:    "golaw: the {agent} hook is installed and trusted, but has not run in this repository yet; use the agent here and run golaw doctor again.",
		codeNotInstalled:       "golaw: the {agent} installation is absent or changed; run {command}.",
		codeTrustUnknown:       "golaw: the {agent} hook is installed; whether the agent runs it depends on a trust doctor cannot read (see docs/agent-contracts.md).",
		codePreviewActive:      "golaw: the {agent} hook is installed and its shell and read guards ran; its writes are captured, not enforced (see docs/agent-contracts.md).",
		summaryAbsent:          "golaw: {agent} is not installed at this scope; to use it, run {command}.",
		summaryFeatureOff:      "golaw: the {agent} hook is installed, but the hooks feature is off in the Codex config.toml; turn features.hooks on and run golaw doctor again.",
	}
}
