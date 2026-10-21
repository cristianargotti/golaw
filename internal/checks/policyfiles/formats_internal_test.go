package policyfiles

import (
	"strings"
	"testing"
)

// gitleaksConfig is a gitleaks configuration with a rule whose regular
// expression is a multi-line literal string, ordinary in gitleaks and the
// reason the family decodes TOML whole instead of through a subset.
const gitleaksConfig = `title = "service"

[extend]
useDefault = true

[[rules]]
id = "internal-token"
regex = '''(?i)token\s*=\s*['"][a-z0-9]{32}['"]'''

[[allowlists]]
paths = ['''^gen/''']
`

func TestDecodeTOMLReadsAGitleaksConfiguration(t *testing.T) {
	t.Parallel()
	var config struct {
		Title  string `toml:"title"`
		Extend struct {
			UseDefault bool `toml:"useDefault"`
		} `toml:"extend"`
		Rules []struct {
			ID    string `toml:"id"`
			Regex string `toml:"regex"`
		} `toml:"rules"`
		Allowlists []struct {
			Paths []string `toml:"paths"`
		} `toml:"allowlists"`
	}
	if err := decodeTOML(".gitleaks.toml", []byte(gitleaksConfig), &config); err != nil {
		t.Fatal(err)
	}
	if config.Title != "service" || !config.Extend.UseDefault || len(config.Rules) != 1 ||
		!strings.HasPrefix(config.Rules[0].Regex, "(?i)token") || len(config.Allowlists) != 1 || config.Allowlists[0].Paths[0] != "^gen/" {
		t.Fatalf("config = %+v", config)
	}
	var decoded map[string]any
	err := decodeTOML(".squawk.toml", []byte("pg_version = \n"), &decoded)
	if err == nil || !strings.HasPrefix(err.Error(), "parse .squawk.toml: ") || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("a syntax error = %v, want the file and the line", err)
	}
}

const twoRequireBlocks = `module example.test/service

go 1.27

require (
	example.test/alpha v1.2.3
	example.test/beta v0.4.0 // indirect
)

require example.test/gamma v2.0.1+incompatible

replace example.test/alpha => ./third_party/alpha
`

func TestParseModuleReadsEveryRequireBlock(t *testing.T) {
	t.Parallel()
	f, err := parseModule("go.mod", []byte(twoRequireBlocks))
	if err != nil {
		t.Fatal(err)
	}
	var direct []string
	for _, r := range f.Require {
		if !r.Indirect {
			direct = append(direct, r.Mod.Path+"@"+r.Mod.Version)
		}
	}
	if strings.Join(direct, ",") != "example.test/alpha@v1.2.3,example.test/gamma@v2.0.1+incompatible" || len(f.Replace) != 1 {
		t.Fatalf("direct = %v, replace = %d", direct, len(f.Replace))
	}
	if got := modulePath([]byte(twoRequireBlocks)); got != "example.test/service" {
		t.Fatalf("modulePath = %q", got)
	}
	if got := modulePath([]byte("go 1.27\n")); got != "" {
		t.Fatalf("modulePath without a module line = %q", got)
	}
	_, err = parseModule("go.mod", []byte("module example.test/service\nrequire (\n"))
	if err == nil || !strings.HasPrefix(err.Error(), "parse go.mod: ") {
		t.Fatalf("a broken go.mod = %v, want a parse error naming the file", err)
	}
}
