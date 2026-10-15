package paths

import (
	"regexp"
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

func TestSecretGlobsLeaveSourceAndDocumentationReadable(t *testing.T) {
	t.Parallel()
	for _, readable := range []string{"credentials.go", "internal/auth/credentials.go", "docs/secrets.md", "config/app.yaml", "cmd/keys.go"} {
		for _, glob := range SecretGlobs() {
			if globMatch(glob, readable) {
				t.Errorf("glob %q denies the readable file %s", glob, readable)
			}
		}
	}
}
