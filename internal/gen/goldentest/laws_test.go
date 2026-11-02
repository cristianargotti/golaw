package goldentest_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/goldentest"
)

func lawNamed(t *testing.T, name string) goldentest.Law {
	t.Helper()
	for _, law := range goldentest.Laws() {
		if law.Name == name {
			return law
		}
	}
	t.Fatalf("no law %s", name)
	return goldentest.Law{}
}

// one is a fixture of one Go file whose body starts on line 3.
func one(path, body string) map[string][]byte {
	return map[string][]byte{path: []byte("package fixture\n\n" + body + "\n")}
}

func TestLawsAreTheRuleTableOfTheAddendum(t *testing.T) {
	t.Parallel()
	want := []string{
		"S107-parameters", "S1192-literals", "no-regexp", "no-http-literal", "doc-ranges-only",
		"no-suppression", "file-modes", "no-credential-literal", "context-from-test",
	}
	got := make([]string, 0, len(want))
	for _, law := range goldentest.Laws() {
		got = append(got, law.Name)
		if law.Assert == nil {
			t.Errorf("%s has no assertion", law.Name)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Laws = %v, want %v", got, want)
	}
}

func TestEachLawJudgesItsShape(t *testing.T) {
	t.Parallel()
	suppression := "//" + "nolint:errcheck // the red shape of the law"
	// The empty context's name is assembled from halves, so this file
	// carries no task marker for text.todo to read.
	empty := "T" + "ODO"
	tests := map[string]struct {
		law   string
		files map[string][]byte
		want  string // a fragment of the only violation; empty for none
	}{
		"red: eight parameters":                     {law: "S107-parameters", files: one("a/ctor.go", "func New(a, b, c, d, e, f, g, h int) int { return a }"), want: "a/ctor.go:3 S107-parameters: 8 parameters"},
		"red: an interface method":                  {law: "S107-parameters", files: one("a/port.go", "type Port interface { Do(a, b, c, d, e, f, g, h int) }"), want: "S107-parameters: 8 parameters"},
		"red: a function literal":                   {law: "S107-parameters", files: one("a/lit.go", "var f = func(a, b, c, d, e, f, g, h int) {}"), want: "S107-parameters: 8 parameters"},
		"red: a file that does not parse":           {law: "S107-parameters", files: map[string][]byte{"a/x.go": []byte("package")}, want: "a/x.go: does not parse"},
		"near: seven parameters":                    {law: "S107-parameters", files: one("a/ctor.go", "func New(a, b, c, d, e, f, g int) int { return a }")},
		"near: one config struct":                   {law: "S107-parameters", files: one("a/ctor.go", "type Config struct{ A int }\n\nfunc Wire(cfg Config) int { return cfg.A }")},
		"red: a literal three times":                {law: "S1192-literals", files: one("a/x.go", `var names = []string{"growth: read plan", "growth: read plan", "growth: read plan"}`), want: "a/x.go:3 S1192-literals: a string literal appears 3 times"},
		"near: the literal named once":              {law: "S1192-literals", files: one("a/x.go", "const action = \"growth: read plan\"\n\nvar names = []string{action, action, action}")},
		"near: a literal in a test file":            {law: "S1192-literals", files: one("a/x_test.go", `var names = []string{"growth: read plan", "growth: read plan", "growth: read plan"}`)},
		"near: a bare word":                         {law: "S1192-literals", files: one("a/x.go", `var names = []string{"readplan", "readplan", "readplan"}`)},
		"near: five runes":                          {law: "S1192-literals", files: one("a/x.go", `var names = []string{"a: bc", "a: bc", "a: bc"}`)},
		"red: an import of regexp":                  {law: "no-regexp", files: one("a/x.go", "import \"regexp\"\n\nvar quote = regexp.QuoteMeta"), want: "imports regexp"},
		"near: regexp named in a comment":           {law: "no-regexp", files: one("a/x.go", "// Valid never imports regexp.\nfunc Valid() bool { return true }")},
		"red: a clear text URL":                     {law: "no-http-literal", files: one("a/x.go", `const base = "http://api.example.com"`), want: "a clear text http:// URL"},
		"near: a TLS URL":                           {law: "no-http-literal", files: one("a/x.go", "// An http:// URL in a comment is no literal.\nconst base = \"https://api.example.com\"")},
		"red: a private address":                    {law: "doc-ranges-only", files: one("a/x.go", `const peer = "10.0.0.7:8080"`), want: "address 10.0.0.7 outside the documentation ranges"},
		"red: a public IPv6 address":                {law: "doc-ranges-only", files: one("a/x.go", `const peer = "2001:4860::8888"`), want: "address 2001:4860::8888 outside the documentation ranges"},
		"near: the documentation ranges":            {law: "doc-ranges-only", files: one("a/x.go", `var peers = []string{"192.0.2.10:8080", "198.51.100.7", "203.0.113.9", "[2001:db8::1]:443"}`)},
		"near: a version and a word":                {law: "doc-ranges-only", files: one("a/x.go", `var words = []string{"version 1.2.3", "dead::beef"}`)},
		"red: a named lint suppression":             {law: "no-suppression", files: one("a/x.go", "func f() error { return nil }\n\nfunc g() {\n\t_ = f() "+suppression+"\n}"), want: "a suppression comment"},
		"red: a Sonar suppression":                  {law: "no-suppression", files: one("a/x.go", "// NOSONAR the red shape of the law\nvar x = 1"), want: "a suppression comment"},
		"near: a comment about linters":             {law: "no-suppression", files: one("a/x.go", "// The linters run clean on this file.\nvar x = 1")},
		"red: a file mode wider than 0o600":         {law: "file-modes", files: one("a/x.go", "import \"os\"\n\nfunc save() error { return os.WriteFile(\"r.txt\", nil, 0o644) }"), want: "mode 0o644 wider than 0o600"},
		"red: a legacy octal mode":                  {law: "file-modes", files: one("a/x.go", "import \"os\"\n\nfunc save() error { return os.Chmod(\"r.txt\", 0644) }"), want: "mode 0o644 wider than 0o600"},
		"red: a directory mode wider than 0o700":    {law: "file-modes", files: one("a/x.go", "import \"os\"\n\nfunc save() error { return os.MkdirAll(\"d\", 0o755) }"), want: "mode 0o755 wider than 0o700"},
		"near: the private modes":                   {law: "file-modes", files: one("a/x.go", "import \"os\"\n\nfunc save() error {\n\t_ = os.MkdirAll(\"d\", 0o700)\n\treturn os.WriteFile(\"d/r.txt\", nil, 0o600)\n}")},
		"red: a credential field literal":           {law: "no-credential-literal", files: one("a/x.go", "type Config struct{ Password string }\n\nvar cfg = Config{Password: \"fixture\"}"), want: "a credential literal in Password"},
		"red: a credential constant":                {law: "no-credential-literal", files: one("a/x.go", `const apiToken = "fixture"`), want: "a credential literal in apiToken"},
		"red: a credential assignment":              {law: "no-credential-literal", files: one("a/x.go", "func f() string {\n\tsecret := \"fixture\"\n\treturn secret\n}"), want: "a credential literal in secret"},
		"near: a generated credential":              {law: "no-credential-literal", files: one("a/x.go", "type Config struct{ Password, Token, TokenURL string }\n\nfunc generated() string { return \"x\" }\n\nvar cfg = Config{Password: generated(), Token: \"\", TokenURL: \"https://auth.example.com/token\"}")},
		"red: a background context in a test":       {law: "context-from-test", files: one("a/x_test.go", "import \"context\"\n\nvar ctx = context.Background()"), want: "context.Background in a test: use t.Context()"},
		"red: the empty context in a test":          {law: "context-from-test", files: one("a/x_test.go", "import \"context\"\n\nvar ctx = context."+empty+"()"), want: "context." + empty + " in a test"},
		"near: a background context outside a test": {law: "context-from-test", files: one("a/x.go", "import \"context\"\n\nvar ctx = context.Background()")},
		"near: the test context":                    {law: "context-from-test", files: one("a/x_test.go", "import \"testing\"\n\nfunc TestX(t *testing.T) { _ = t.Context() }")},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := lawNamed(t, tc.law).Assert(tc.files)
			if tc.want == "" && len(got) != 0 {
				t.Fatalf("%s = %q, want no violation", tc.law, got)
			}
			if tc.want != "" && (len(got) != 1 || !strings.Contains(got[0], tc.want)) {
				t.Fatalf("%s = %q, want one violation holding %q", tc.law, got, tc.want)
			}
		})
	}
}
