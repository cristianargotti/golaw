package assets_test

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/assets"
)

const (
	ciAssetPath            = ".github/workflows/ci.yml"
	ciAssetSource          = "templates/.github/workflows/ci.yml.tmpl"
	ciEnrollmentExpression = "${{ vars.GOLAW_ENROLLED }}"
)

func ciCapabilitySets() [][]string {
	optional := []string{"api", "cli", "db", "hexagonal", "image"}
	sets := make([][]string, 0, 2+(1<<len(optional)))
	sets = append(sets, nil, []string{"image"})
	for mask := range 1 << len(optional) {
		caps := []string{"go"}
		for bit, capability := range optional {
			if mask&(1<<bit) != 0 {
				caps = append(caps, capability)
			}
		}
		slices.Sort(caps)
		sets = append(sets, caps)
	}
	return sets
}

func TestCITemplateCapabilityJobs(t *testing.T) {
	t.Parallel()
	for _, caps := range ciCapabilitySets() {
		t.Run(strings.Join(caps, ","), func(t *testing.T) {
			t.Parallel()
			requireCIAsset(t, caps)
		})
	}
}

func requireCIAsset(t *testing.T, caps []string) {
	t.Helper()
	context := assetContext()
	context.Capabilities = caps
	first := renderedAsset(t, ciAssetPath, context)
	second := renderedAsset(t, ciAssetPath, context)
	if !bytes.Equal(first.Data, second.Data) {
		t.Fatal("CI rendering is not deterministic")
	}
	if first.Template.Source != ciAssetSource ||
		first.Template.Ownership != assets.OwnedByKit ||
		len(first.Template.When) != 0 {
		t.Fatalf("CI manifest entry = %+v", first.Template)
	}
	body := string(first.Data)
	if lines := strings.Count(strings.TrimSpace(body), "\n") + 1; lines > 300 {
		t.Fatalf("rendered CI has %d lines", lines)
	}
	var root map[string]any
	if err := yaml.Unmarshal(first.Data, &root); err != nil {
		t.Fatal(err)
	}
	wantJobs := requireCIJobs(t, yamlMap(t, root, "jobs"), caps)
	requireCITools(t, body, caps)
	if got := strings.Count(body, "actions/setup-go@"); got != wantJobs {
		t.Fatalf("Go setups=%d, want one per job, %d", got, wantJobs)
	}
	requireCICommands(t, body)
}

func requireCIJobs(t *testing.T, jobs map[string]any, caps []string) int {
	t.Helper()
	if _, ok := jobs["ci"]; !ok {
		t.Fatal("missing ci job")
	}
	wantJobs := 1
	for _, capability := range []string{"api", "db", "image", "hexagonal"} {
		_, present := jobs[capability]
		enabled := slices.Contains(caps, capability)
		if present != enabled {
			t.Fatalf("job %s present=%t for %v", capability, present, caps)
		}
		if enabled {
			wantJobs++
		}
	}
	if len(jobs) != wantJobs {
		t.Fatalf("jobs=%d, want %d", len(jobs), wantJobs)
	}
	return wantJobs
}

func requireCITools(t *testing.T, body string, caps []string) {
	t.Helper()
	hasGo := slices.Contains(caps, "go")
	for _, test := range []struct {
		text string
		on   bool
	}{
		{"actions/setup-go@", true},
		{"cmd/golaw@", true},
		{"go-version-file: go.mod", hasGo},
		{`go-version: "1.27.0", cache: false`, !hasGo},
		{"golangci-lint@", hasGo},
		{"gitleaks/v8@", hasGo},
		{"govulncheck@", hasGo},
		{"@redocly/cli@", slices.Contains(caps, "api")},
		{"squawk-cli@", slices.Contains(caps, "db")},
		{"hadolint-linux-x86_64", slices.Contains(caps, "image")},
		{"trivy_0.75.0_Linux-64bit", slices.Contains(caps, "image")},
		{"sha256sum -c -", slices.Contains(caps, "image")},
	} {
		if strings.Contains(body, test.text) != test.on {
			t.Errorf("%q does not follow its capability", test.text)
		}
	}
}

func requireCICommands(t *testing.T, body string) {
	t.Helper()
	for _, forbidden := range []string{
		"golaw setup", "lefthook install", "@latest",
		"make mutation-changed", "make fuzz", "make bench",
		"make test-integration", "GOLAW_ENROLLED:?",
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("unexpected CI command: %s", forbidden)
		}
	}
	if !strings.Contains(body, ciEnrollmentExpression) {
		t.Fatal("CI does not consume external enrollment")
	}
}
