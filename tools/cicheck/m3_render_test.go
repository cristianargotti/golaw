package cicheck_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/runner"
)

const (
	m3CIPath      = ".github/workflows/ci.yml"
	m3Enrollment  = "${{ vars.GOLAW_ENROLLED }}"
	m3GateCommand = "make ci"
)

func m3Capabilities() [][]string {
	optional := []string{"api", "cli", "db", "hexagonal", "image"}
	sets := make([][]string, 0, 2+(1<<len(optional)))
	sets = append(sets, nil, []string{"image"})
	for mask := range 1 << len(optional) {
		capabilities := []string{"go"}
		for bit, capability := range optional {
			if mask&(1<<bit) != 0 {
				capabilities = append(capabilities, capability)
			}
		}
		slices.Sort(capabilities)
		sets = append(sets, capabilities)
	}
	return sets
}

func m3CI(t *testing.T, capabilities []string) []byte {
	t.Helper()
	tools := make(map[string]string)
	for _, tool := range runner.Toolchain() {
		tools[tool.Name] = tool.Version
	}
	values := assets.Values{
		ModulePath: "example.test/service", GoVersion: "1.27", KitVersion: "0.2.0",
		Language: "pt-BR", Decisions: "docs/adr", Main: "root",
		Capabilities: capabilities, Tools: tools,
	}
	context := assets.Context{Values: values, CIBackend: "none"}
	rendered, err := assets.RenderAllContext(golaw.FS, context)
	if err != nil {
		t.Fatal(err)
	}
	for i := range rendered {
		if rendered[i].Path == m3CIPath {
			return rendered[i].Data
		}
	}
	t.Fatalf("missing rendered destination %s", m3CIPath)
	return nil
}

func TestM3ProvisioningBoundaries(t *testing.T) {
	t.Parallel()
	const scanner = "go install github.com/cristianargotti/golaw/cmd/golaw@v0.2.0"
	for _, test := range []struct {
		name, old, next string
		caps            []string
		fail            bool
	}{
		{"quoted cache", "cache: false", `cache: "false"`, nil, true},
		{"wrong standalone Go", `go-version: "1.27.0"`, `go-version: "1.26.0"`, nil, true},
		{"standalone module reference", "cache: false", "cache: false, go-version-file: go.mod", nil, true},
		{"scanner unpinned", scanner, strings.Replace(scanner, "v0.2.0", "latest", 1), nil, true},
		{"scanner echoed", scanner, "echo " + scanner, nil, true},
		{"scanner duplicated", scanner, scanner + "\n      - run: " + scanner, nil, true},
		{"empty extra Go tool", scanner, scanner + "\n      - run: go install example.test/tool@v1.0.0", nil, true},
		{"empty npm tool", scanner, scanner + "\n      - run: npm install --global squawk-cli@2.35.0", nil, true},
		{"empty download", scanner, scanner + "\n      - run: curl -fsSL https://example.test/tool", nil, true},
		{"Go without module", "go-version-file: go.mod", `go-version: "1.27.0", cache: false`, []string{"go"}, true},
		{"Go tool missing", "go install golang.org/x/vuln/cmd/govulncheck@v1.7.0", "go version", []string{"go"}, true},
		{"Go tool echoed", "go install golang.org/x/vuln/cmd/govulncheck@v1.7.0", "echo go install golang.org/x/vuln/cmd/govulncheck@v1.7.0", []string{"go"}, true},
		{"api tool missing", "npm install --global @redocly/cli@2.5.0", "node --version", []string{"api", "go"}, true},
		{"db tool missing", "npm install --global squawk-cli@2.35.0", "node --version", []string{"db", "go"}, true},
		{"image tool missing", "curl -fsSL https://github.com/hadolint/", "echo https://github.com/hadolint/", []string{"image"}, true},
		{"metadata", "name: CI", "name: Verification", nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			source := string(m3CI(t, test.caps))
			changed := strings.Replace(source, test.old, test.next, 1)
			if changed == source {
				t.Fatal("provisioning mutation did not change its source")
			}
			if reason := inspectM3Provisioning([]byte(changed), test.caps); (reason != "") != test.fail {
				t.Fatalf("reason=%q, want failure=%t", reason, test.fail)
			}
		})
	}
}

func TestM3ProvisioningOrderAndJobs(t *testing.T) {
	t.Parallel()
	const scanner = "go install github.com/cristianargotti/golaw/cmd/golaw@v0.2.0"
	source := string(m3CI(t, []string{"image"}))
	early := strings.Replace(source, "- run: "+scanner, "- run: go version", 1)
	early = strings.Replace(early, "- uses: actions/setup-go@", "- run: "+scanner+"\n      - uses: actions/setup-go@", 1)
	before, after, found := strings.CutLast(source, scanner)
	if !found {
		t.Fatal("image rendering does not install golaw")
	}
	withoutOptional := before + "go version" + after
	for name, changed := range map[string]string{"early scanner": early, "optional scanner missing": withoutOptional} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if reason := inspectM3Provisioning([]byte(changed), []string{"image"}); reason == "" {
				t.Fatal("missing per-job provisioning failure")
			}
		})
	}
}
