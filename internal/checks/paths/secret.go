package paths

import (
	"regexp"
	"strings"
)

// sourceFile marks a hand-written source or documentation file, never secret
// material by name: gitleaks reads its content.
var sourceFile = regexp.MustCompile(`(?i)\.(?:go|ts|tsx|js|mjs|cjs|py|md|sql|proto|tmpl)$`)

// exampleFile marks a template that documents the shape of a secret file
// and holds no value.
var exampleFile = regexp.MustCompile(`(?i)\.(?:example|sample|template|dist)$`)

// secretPattern joins the name list into one case-insensitive match over
// the end of a slash separated path.
var secretPattern = regexp.MustCompile(`(?i)(?:^|/)(?:` + strings.Join(secretNames(), "|") + `)$`)

// secretNames is the name list of section 8.4 of the specification as data;
// every entry matches the end of a slash separated path.
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

// secretName reports whether a path names secret material: a source or
// documentation file never does, a documented template never does, and
// otherwise the name list decides.
func secretName(path string) bool {
	base := path[strings.LastIndex(path, "/")+1:]
	if sourceFile.MatchString(base) || exampleFile.MatchString(base) {
		return false
	}
	return secretPattern.MatchString(path)
}
