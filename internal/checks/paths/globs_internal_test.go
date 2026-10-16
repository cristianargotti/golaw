package paths

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// sampleOf builds a path a glob covers: a home segment for the tilde, two
// directories for a double star and x for a star.
func sampleOf(glob string) string {
	s := strings.ReplaceAll(glob, "~/", "home/")
	s = strings.ReplaceAll(s, "**/", "a/b/")
	s = strings.ReplaceAll(s, "/**", "/a/b")
	return strings.ReplaceAll(s, "*", "x")
}

// globMatch is the minimal reading of the glob grammar the deny rules use:
// a double star spans directories, a star stays within one segment and a
// tilde is the home directory.
func globMatch(glob, path string) bool {
	expr := regexp.QuoteMeta(strings.TrimPrefix(glob, "~/"))
	expr = strings.ReplaceAll(expr, `\*\*/`, `(?:.*/)?`)
	expr = strings.ReplaceAll(expr, `/\*\*`, `(?:/.*)?`)
	expr = strings.ReplaceAll(expr, `\*`, `[^/]*`)
	if strings.HasPrefix(glob, "~/") {
		expr = "home/" + expr
	}
	return regexp.MustCompile("^" + expr + "$").MatchString(path)
}

func TestSecretGlobsAgreeWithTheNameList(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, glob := range SecretGlobs() {
		if glob == "" || seen[glob] || strings.ContainsAny(glob, "()") || strings.HasSuffix(glob, ".go") {
			t.Errorf("glob %q cannot become a deny rule", glob)
		}
		seen[glob] = true
		sample := sampleOf(glob)
		if !globMatch(glob, sample) {
			t.Errorf("glob %q does not cover its own sample %s", glob, sample)
		}
		if !secretName(sample) {
			t.Errorf("glob %q names %s, which the check does not judge as secret material", glob, sample)
		}
	}
}

// secretPaths are the paths the deny rules must cover, held apart from the
// globs so that a list that loses an entry, or every entry, turns red: one
// or more per family of the name list, with the stage variants of the
// dotenv and kubeconfig files and the Vault token inside a project.
func secretPaths() []string {
	return []string{
		".env", "app/.env.local", "prod.env", ".envrc",
		".env.development", "config/.env.staging", ".env.production", ".env.test", ".env.dev", ".env.stage", ".env.prod",
		"secrets/db.yaml", "deploy/secret/token.txt", "credentials/app.json", "keys/signing.json",
		"config/secrets.json", "app/secrets.yml", "aws-credentials.csv", "gcp-credentials.json",
		"kubeconfig", "kubeconfig.prod", "deploy/kubeconfig.yaml", "kubeconfig.yml", "kubeconfig.json", "cluster.kubeconfig",
		".kube/config", "home/.kube/config", ".docker/config.json", "home/.docker/config.json",
		".vault-token", "deploy/.vault-token", "home/.vault-token",
		"terraform.tfstate", "terraform.tfstate.backup", "prod.tfvars", "prod.tfvars.json",
		"deploy.pem", "server.key", "store.p12", "cert.pfx", "trust.jks", "vault.kdbx",
		"id_rsa", "keys/id_ed25519", "home/.ssh/id_ecdsa",
		".aws/credentials", "home/.aws/credentials", "service-account.json", "sa_service_account.json",
		"home/.config/gcloud/adc.json", "home/.azure/accessTokens.json",
		".netrc", "_netrc", "home/.npmrc", "home/.pypirc", "home/.bashrc", "home/.zshrc", "home/.profile",
	}
}

func TestSecretGlobsCoverTheSecretPaths(t *testing.T) {
	t.Parallel()
	globs := SecretGlobs()
	for _, path := range secretPaths() {
		if !secretName(path) {
			t.Errorf("%s is not secret material for the check", path)
		}
		if !slices.ContainsFunc(globs, func(glob string) bool { return globMatch(glob, path) }) {
			t.Errorf("no glob denies %s", path)
		}
	}
}

func TestSecretGlobsLeaveSourceAndDocumentationReadable(t *testing.T) {
	t.Parallel()
	readables := []string{
		"credentials.go", "internal/auth/credentials.go", "docs/secrets.md", "config/app.yaml", "cmd/keys.go",
		".env.example", "config/.env.sample", ".env.template", "kubeconfig.example",
		"internal/kube/kubeconfig.go", "docs/kubeconfig.md",
	}
	for _, readable := range readables {
		for _, glob := range SecretGlobs() {
			if globMatch(glob, readable) {
				t.Errorf("glob %q denies the readable file %s", glob, readable)
			}
		}
	}
}
