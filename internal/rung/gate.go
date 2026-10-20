package rung

import (
	"context"
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/report"
)

// armed hands the rung the policy and the ladder the gate read, so the
// rung judges with the very ones that armed it.
type armed struct {
	loaded *policy.Loaded
	ladder model.Ladder
}

// gate does what every rung does before its own work (decision 10, A3,
// A10). The ladder of the golaw.yml the checkout holds switches the rung
// off before anything else, so a rung switched off never refuses for a
// version, a tool, a message or an input; the policy in force loads with
// the pin and the enrollment checked; a positively unenrolled location,
// without a record and without golaw.yml, answers nothing; and when the
// checkout holds no golaw.yml the ladder of the policy in force decides. It
// answers the armed rung, or the outcome git gets instead.
func gate(ctx context.Context, deps Dependencies, name string) (armed, Outcome, bool) {
	ladder, local := policy.CheckoutLadder(deps.Dir)
	if local && !on(ladder, name) {
		return armed{}, Outcome{}, false
	}
	loaded, err := deps.Load(ctx, deps.Dir)
	if err != nil {
		return armed{}, failed(name, model.DefaultLanguage, err), false
	}
	if loaded.Record == nil && loaded.Origin == policy.OriginCore {
		return armed{}, Outcome{}, false
	}
	if !local {
		ladder = loaded.Policy.Ladder
	}
	if !on(ladder, name) {
		return armed{}, Outcome{}, false
	}
	return armed{loaded: loaded, ladder: ladder}, Outcome{}, true
}

// on reads the switch of the named rung; a name the ladder does not know
// is off, so it never runs.
func on(ladder model.Ladder, name string) bool {
	switch name {
	case PreCommit:
		return ladder.PreCommit
	case CommitMsg:
		return ladder.CommitMsg
	case PrePush:
		return ladder.PrePush
	default:
		return false
	}
}

// failed refuses with the failure code when a rung could not judge: the
// English reason first, for the person and the agent alike, then one line
// in the language of the policy that points the person at it.
func failed(name, language string, err error) Outcome {
	return Outcome{Code: codeFailure, Lines: []string{prefix(name) + err.Error(), refused(language, name)}}
}

// prefix starts every line a rung writes itself, so a person tells its
// lines apart from the output of the tools it runs.
func prefix(name string) string {
	return "golaw " + name + ": "
}

// refused is the human line of a refusal no phrase of the report names: in
// Portuguese for pt-BR and in English for any other language, as
// report.Human chooses, pointing at the English lines above it.
func refused(language, name string) string {
	if language == report.Portuguese {
		return fmt.Sprintf("golaw recusou o hook %s; a causa e a correção estão nas linhas acima.", name)
	}
	return fmt.Sprintf("golaw refused the %s hook; the lines above name the cause and the fix.", name)
}
