package toolchain

import (
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
)

// MissingNotice answers the agent sentence, then the person sentence, of
// a pinned tool the machine does not hold (M6.5.3). human is the configured
// language of the human channel, never the surface language of the file:
// Portuguese for pt-BR, English for any other. The hook starts only the
// tools setup verified (A28), so the sentence names the kit tool directory:
// a tool doctor finds on PATH at its pin is still one the hook leaves alone.
func MissingNotice(t Tool, path, human string) (string, string) {
	agent := fmt.Sprintf("%s %s is not installed in the kit tool directory: %s was judged by the native checks only; run golaw setup --surface, or the pipeline judges the rest.", t.Name, t.Version, path)
	if human != "" && human != model.DefaultLanguage {
		return agent, agent
	}
	person := fmt.Sprintf("%s %s não está instalado no diretório de ferramentas do kit: o golaw julgou %s só com as verificações nativas; rode golaw setup --surface ou deixe o resto para o pipeline.", t.Name, t.Version, path)
	return agent, person
}
