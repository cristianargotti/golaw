package shell

import (
	"regexp"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// holderName is the shape of a variable name that holds a credential.
const holderName = `[A-Za-z0-9_${}]*(?:SECRET|TOKEN|PASSWORD|PASSWD|CREDENTIAL|API_?KEY|PRIVATE_?KEY)[A-Za-z0-9_}]*`

// credentialPrinter matches a whole unwrapped segment that prints a
// credential.
var credentialPrinter = regexp.MustCompile(`(?i)^(?:` + strings.Join(printers(), "|") + `)$`)

// printers is the command list of section 8.4 as data: a full dump of the
// environment, a named credential variable, and the tool commands that read
// a stored secret.
func printers() []string {
	return []string{
		`printenv`, `env`,
		`(?:printenv|echo) .*` + holderName,
		`gh auth token`,
		`security find-generic-password .*-w\b.*`,
		`aws configure get .*`,
		`op read .*`, `vault read .*`, `vault kv get .*`,
		`kubectl get secrets? .*(?:-o|--output)\b.*`,
	}
}

// CredentialPrint is shell.credential-print: no command prints a credential
// into the context.
func CredentialPrint() model.Check {
	return commandRule{spec: hookSpec("shell.credential-print"), judge: judgeCredentialPrint}
}

func judgeCredentialPrint(_ model.Input, segs []segment) ([]model.Finding, error) {
	for _, seg := range segs {
		if credentialPrinter.MatchString(strings.Join(unwrap(seg), " ")) {
			return []model.Finding{deny("credential-print", "the command prints a credential; secrets never enter the context")}, nil
		}
	}
	return nil, nil
}
