package hook

import (
	"fmt"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

// TestProfileRefusalSpeaksToThePersonInTheirLanguage pins the human channel
// of D8: a denial whose reason is a refusal of the pinned profile bundle
// tells the person which profile and the exact command, in the configured
// language, while the English reason the agent reads stays byte for byte.
func TestProfileRefusalSpeaksToThePersonInTheirLanguage(t *testing.T) {
	t.Parallel()
	const command = "golaw profile install https://example.test/profiles/fixture@1.2.3"
	chain := "load hook policy: load the policy of /work/app: invalid policy: "
	reasons := map[string]string{
		"missing":       chain + "profile fixture is not installed: run " + command,
		"stale":         chain + "profile fixture: version is 1.2.3 in golaw.yml and 1.3.0 in the installed bundle: run " + command,
		"other digest":  chain + "profile fixture: digest is sha256:aa in golaw.yml and sha256:bb in the installed bundle: run " + command,
		"files changed": chain + "profile digest mismatch: profile fixture: digest sha256:cc of the installed bundle differs from its stamp sha256:aa: run " + command,
	}
	event := model.Event{Shape: model.ShapeWrite}
	for name, reason := range reasons {
		got := failureLanguage(event, fmt.Errorf("%w: %s", model.ErrPolicy, reason), model.DefaultLanguage)
		if got.Outcome != Deny || got.Reason != model.ErrPolicy.Error()+": "+reason || got.CannotJudge {
			t.Errorf("%s: result = %+v", name, got)
		}
		if want := "Perfil fixture ausente ou divergente: execute " + command; got.Human != want {
			t.Errorf("%s: human = %q, want %q", name, got.Human, want)
		}
		if english := humanMessage("en", event, reason); english != "Profile fixture is missing or differs from its pin: run "+command {
			t.Errorf("%s: english = %q", name, english)
		}
	}
}

// TestOtherRefusalsKeepTheirSentence pins the near misses: a schema cause
// names no command, and a command alone names no profile, so both keep the
// sentence every other denial carries.
func TestOtherRefusalsKeepTheirSentence(t *testing.T) {
	t.Parallel()
	event := model.Event{Shape: model.ShapeWrite}
	for _, reason := range []string{
		"invalid policy: invalid profile bundle: profile fixture: rules.yml: terms: \"Bad\" is not lower case kebab case",
		"invalid policy: the pipeline asks to run golaw profile install https://example.test/x@1.0.0",
		"invalid policy: golaw.yml pins kit version 0.0.1 and this binary is 9.9.9: install the pinned version or run init --update",
	} {
		if got, generic := humanMessage(model.DefaultLanguage, event, reason), "A operação foi bloqueada pela verificação golaw no caminho"; len(got) < len(generic) || got[:len(generic)] != generic {
			t.Errorf("%q: human = %q", reason, got)
		}
	}
}
