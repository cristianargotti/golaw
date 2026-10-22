package doctor

import "strings"

// The codes of the trusted line a Codex handler can hold besides trusted
// and trust-unknown (A20).
const (
	codeUntrusted    = "untrusted"
	codeRevalidation = "revalidation-required"
)

// codexTrustLine says whether Codex will run the installed handlers: a
// handler changed since it was trusted needs revalidation, one without a
// trusted hash at its key is untrusted, and so is every handler when the
// hooks feature is off or the project layer is not trusted; never trusted
// on a difference.
func codexTrustLine(f agentFacts) Line {
	line := Line{ID: idTrusted, Key: f.scopeKey(), State: stateUnknown, Code: codeTrustUnknown}
	trust := f.Trust
	modified, untrusted := trustKeys(trust.Hooks, statusModified), trustKeys(trust.Hooks, statusUntrusted, statusAbsent)
	disabled := trustKeys(trust.Hooks, statusDisabled)
	switch {
	case f.Receipt.State != receiptRead:
		line.Detail = "nothing is installed for Codex to trust"
	case trust.Source == "":
		line.Detail = "the Codex trust is not read"
	case trust.Error != "":
		line.Detail = "the trust record cannot be read: " + trust.Error
	case trust.Feature == featureOff:
		line.State, line.Code = stateRed, codeUntrusted
		line.Detail = "the hooks feature is off in " + trust.Config + ", so Codex runs no hook"
	case trust.Project == projectUntrusted:
		line.State, line.Code = stateRed, codeUntrusted
		line.Detail = "Codex loads the project layer only once the project is trusted: trust the repository in Codex, then approve the hooks with /hooks"
	case len(disabled) > 0:
		line.State, line.Code = stateRed, codeUntrusted
		line.Detail = "turned off with enabled = false in " + trust.Config + ", so Codex skips them: " + strings.Join(disabled, ", ") + "; turn them on again in Codex with /hooks"
	case len(modified) > 0:
		line.State, line.Code = stateRed, codeRevalidation
		line.Detail = "changed since they were trusted, so Codex skips them: " + strings.Join(modified, ", ") + "; review them in Codex with /hooks"
	case len(untrusted) > 0:
		line.State, line.Code = stateRed, codeUntrusted
		line.Detail = "not trusted, so Codex skips them: " + strings.Join(untrusted, ", ") + "; approve them in Codex with /hooks"
	case trust.Project == projectAncestor:
		line.Detail = "only an ancestor of the repository is a trusted Codex project; whether Codex loads this project layer is unknown"
	default:
		line.State, line.Code = stateOK, codeTrusted
		line.Detail = "Codex trusts every installed handler"
	}
	if trust.Source != "" {
		line.Detail += " (" + trustSource(trust) + ")"
	}
	return line
}

// trustKeys are the keys of the handlers at any of the statuses.
func trustKeys(hooks []hookTrust, statuses ...string) []string {
	var out []string
	for _, hook := range hooks {
		for _, status := range statuses {
			if hook.Status == status {
				out = append(out, hook.Key)
			}
		}
	}
	return out
}

// trustSource names where the trust was read and, for the record, why
// Codex itself was not asked.
func trustSource(trust trustFact) string {
	switch {
	case trust.Source != sourceRecord:
		return "from " + trust.Source
	case trust.Reason != "":
		return "from the trust record in " + trust.Config + "; codex app-server did not answer: " + trust.Reason
	default:
		return "from the trust record in " + trust.Config
	}
}
