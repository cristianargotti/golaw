package report

// policyLintPhrases are the sentences of the codes the lint half of the
// policy family emits (policy.golangci, policy.ruleguard,
// policy.gitleaks-generic, policy.gotoolchain-local), one per code, in the
// order its checks register.
func policyLintPhrases() []sentence {
	return []sentence{
		{code: "golangci-missing", text: "configuração do golangci-lint ausente na raiz; gere o .golangci.yml com o golaw init"},
		{code: "golangci-linter-off", text: "linter exigido pela política não está ativo; ative-o em linters.enable"},
		{code: "golangci-linter-disabled", text: "linter exigido pela política está em linters.disable; retire-o da lista"},
		{code: "golangci-hollowed", text: "regra de exclusão esvazia um linter exigido; remova a regra"},
		{code: "golangci-exclusion-unlisted", text: "exclusão de um linter exigido fora das exclusões permitidas do golaw.yml; remova a exclusão"},
		{code: "golangci-shadow-off", text: "o govet não roda a análise shadow; ative enable-all ou acrescente shadow"},
		{code: "golangci-layer-missing", text: "nenhuma regra estrita do depguard protege uma camada interna; acrescente a regra que o golaw init gera"},
		{code: "golangci-layer-open", text: "regra do depguard abre uma camada interna; deixe a regra estrita e permita só a biblioteca padrão e as camadas internas"},
		{code: "golangci-float32-allowed", text: "o forbidigo não proíbe float32; acrescente o padrão ^float32$"},
	}
}
