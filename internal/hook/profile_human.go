package hook

import (
	"fmt"
	"regexp"

	"github.com/cristianargotti/golaw/internal/model"
)

// profileRemedy finds, in the reason of a refused load, the profile whose
// bundle is absent or differs and the command that installs its pin, as
// the policy and profile packages word every such refusal (A19, D8).
var profileRemedy = regexp.MustCompile(`profile ([a-z0-9][a-z0-9-]*)(?::| is not installed).*: run (golaw profile install \S+)$`)

// profileHuman is the line a person reads when the hook denies because the
// pinned profile bundle is absent or differs: the profile and the exact
// command in the configured language, never the English chain of the load.
func profileHuman(language, reason string) (string, bool) {
	match := profileRemedy.FindStringSubmatch(reason)
	if match == nil {
		return "", false
	}
	if language == model.DefaultLanguage {
		return fmt.Sprintf("Perfil %s ausente ou divergente: execute %s", match[1], match[2]), true
	}
	return fmt.Sprintf("Profile %s is missing or differs from its pin: run %s", match[1], match[2]), true
}
