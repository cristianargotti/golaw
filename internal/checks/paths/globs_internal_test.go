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
// globs so that a list that loses an entry, or every entry, turns red: the
// families of section 8.2 the rules close the reference gap for.
func secretPaths() []string {
	return []string{
		"deploy.pem", "server.key", "store.p12", "cert.pfx", "trust.jks", "vault.kdbx",
		"terraform.tfvars", "infra/terraform.tfvars.json", "prod.auto.tfvars", "infra/prod.auto.tfvars.json",
		"home/.ssh/id_rsa", "home/.ssh/id_dsa", "home/.ssh/id_ecdsa", "home/.ssh/id_ed25519",
		"home/.aws/credentials", "home/.config/gcloud/adc.json", "home/.azure/accessTokens.json",
		"service-account.json", "deploy/sa_service_account.json",
	}
}

// TestSecretGlobsStayNarrow holds the installer to the narrow rules of
// section 8.2: the rules land in the user settings, which the host applies
// to every project, enrolled or not, before any hook runs, so the broad
// name list stays with the hook, which judges each read in its repository.
func TestSecretGlobsStayNarrow(t *testing.T) {
	t.Parallel()
	narrow := regexp.MustCompile(`^(?:\*\*/\*\.(?:pem|key|p12|pfx|jks|kdbx)|\*\*/(?:terraform|\*\.auto)\.tfvars(?:\.json)?|~/\.ssh/id_(?:rsa|dsa|ecdsa|ed25519)|~/\.aws/credentials|~/\.config/gcloud/\*\.json|~/\.azure/\*\*|\*\*/\*service[-_]account\.json)$`)
	for _, glob := range SecretGlobs() {
		if !narrow.MatchString(glob) {
			t.Errorf("glob %q is broader than the families of section 8.2", glob)
		}
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

func TestSecretGlobsLeaveTemplatesReadable(t *testing.T) {
	t.Parallel()
	templates := []string{
		"dev.example.tfvars", "terraform.tfvars.example", "infra/prod.sample.tfvars",
		"service-account.example.json", "config/service_account.sample.json",
	}
	for _, template := range templates {
		if secretName(template) {
			t.Errorf("the check denies the template %s", template)
		}
		if slices.ContainsFunc(SecretGlobs(), func(glob string) bool { return globMatch(glob, template) }) {
			t.Errorf("a glob denies the template %s", template)
		}
	}
}

func TestSecretGlobsLeaveOtherSecretNamesToTheHook(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"prod.tfvars", "infra/staging.tfvars.json", "service-account-prod.json"} {
		if !secretName(path) {
			t.Errorf("the check allows the secret material %s", path)
		}
		if slices.ContainsFunc(SecretGlobs(), func(glob string) bool { return globMatch(glob, path) }) {
			t.Errorf("a glob denies %s instead of leaving it to the hook", path)
		}
	}
}

func TestSecretGlobsLeaveSourceAndDocumentationReadable(t *testing.T) {
	t.Parallel()
	readables := []string{
		"credentials.go", "internal/auth/credentials.go", "docs/secrets.md", "config/app.yaml", "cmd/keys.go",
		".env.example", "config/.env.sample", ".env.template", "kubeconfig.example",
		"internal/kube/kubeconfig.go", "docs/kubeconfig.md",
		"secrets/client.go", "secret/client.go", "internal/secret/secret.go", "internal/secrets/vault.go",
		"internal/secrets/testdata/payload.json", "charts/app/templates/secret.yaml",
	}
	for _, readable := range readables {
		for _, glob := range SecretGlobs() {
			if globMatch(glob, readable) {
				t.Errorf("glob %q denies the readable file %s", glob, readable)
			}
		}
	}
}
