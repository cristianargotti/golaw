package shell

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// The name list of section 8.4 of the specification, as the paths family
// carries it; the two families cannot share code because each imports the
// model only, so the list is data in both, a test of this package holds the
// two equal and the proof fixtures keep their behavior aligned.
var (
	sourceFile    = regexp.MustCompile(`(?i)\.(?:go|ts|tsx|js|mjs|cjs|py|md|sql|proto|tmpl)$`)
	exampleFile   = regexp.MustCompile(`(?i)\.(?:example|sample|template|dist)$`)
	secretPattern = regexp.MustCompile(`(?i)(?:^|/)(?:` + strings.Join(secretNames(), "|") + `)$`)
)

// secretNames is the name list as data; every entry matches the end of a
// slash separated path.
func secretNames() []string {
	return []string{
		`\.env(?:\.[^/]+)?`, `[^/]+\.env`, `\.envrc`,
		`secrets?/.+`,
		`(?:credentials|keys)/[^/]+\.json`,
		`[^/]*secrets?\.(?:json|ya?ml|env|txt|ini|cfg|conf|csv)`,
		`kubeconfig(?:\.[^/]+)?`, `[^/]+\.kubeconfig`, `\.kube/.+`,
		`\.docker/config\.json`, `\.vault-token`,
		`[^/]+\.tfstate(?:\.backup)?`, `[^/]+\.tfvars(?:\.json)?`,
		`[^/]+\.(?:pem|key|p12|pfx|jks|kdbx)`,
		`id_(?:rsa|dsa|ecdsa|ed25519)`,
		`\.aws/credentials`, `[^/]*service[-_]?account[^/]*\.json`, `gcloud/[^/]+\.json`, `\.azure/.+`,
		`\.netrc`, `_netrc`, `\.npmrc`, `\.pypirc`,
		`\.(?:bashrc|bash_profile|zshrc|zprofile|zshenv|profile)`,
		`[^/]*credentials[^/]*(?:\.(?:json|csv|txt|ya?ml|ini|cfg|conf|env))?`,
	}
}

// secretName reports whether a token names secret material: a source or
// documentation file never does, a documented template never does, and
// otherwise the name list decides.
func secretName(token string) bool {
	name := token[strings.LastIndex(token, "/")+1:]
	if sourceFile.MatchString(name) || exampleFile.MatchString(name) {
		return false
	}
	return secretPattern.MatchString(token)
}

// hookRule is the rule reference of the hook denials, until docs/rules.md
// ships.
const hookRule = "rules/4.7@1"

// hookSpec builds the specification of a hook denial check: event scope, the
// core owner, the hook rule and the proof key equal to the id.
func hookSpec(id string) model.Spec {
	return model.Spec{ID: id, Rule: hookRule, Scope: model.ScopeEvent, Owner: model.OwnerCore, Proof: id}
}

// SecretTokens is shell.secret-tokens: no command names secret material;
// the tokens of the command line are judged by name, never read.
func SecretTokens() model.Check {
	return commandRule{spec: hookSpec("shell.secret-tokens"), judge: judgeSecretTokens}
}

// privateKey matches the private key names a command may carry as a bare
// word. Every other name of the list needs a slash or a dot to be judged,
// because a bare credentials or kubeconfig is as often a search term, a
// branch or a word of a message as a file, while no such word is id_rsa.
var privateKey = regexp.MustCompile(`(?i)^id_(?:rsa|dsa|ecdsa|ed25519)$`)

// secretToken reports whether a token of a command names secret material:
// a path or a file name through the name list, a bare word only when it is
// a private key name.
func secretToken(token string) bool {
	if strings.ContainsAny(token, "/.") {
		return secretName(token)
	}
	return privateKey.MatchString(token)
}

func judgeSecretTokens(_ model.Input, segs []segment) ([]model.Finding, error) {
	for _, seg := range segs {
		for _, word := range seg {
			for token := range strings.SplitSeq(word, "=") {
				token = strings.TrimLeft(token, "<>")
				if secretToken(token) {
					message := fmt.Sprintf("the command names secret material (%s); secrets never enter the context", token)
					return []model.Finding{deny("secret-token", message)}, nil
				}
			}
		}
	}
	return nil, nil
}
