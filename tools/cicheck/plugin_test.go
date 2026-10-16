package cicheck_test

import (
	"os"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/proof"
)

func TestPluginJob(t *testing.T) {
	t.Parallel()
	agent := agentID(t)
	data := kitBytes(t, ".github/workflows/ci.yml")
	if reason := inspectPlugin(agent, data); reason != "" {
		t.Fatal(reason)
	}
	fixture, err := os.ReadFile("proof/testdata/ci.plugin/near/.github/workflows/ci.yml.esc")
	if err != nil {
		t.Fatal(err)
	}
	source := string(proof.Unescape(fixture))
	for _, mutation := range [][2]string{
		{"2.1.299", "2.1.294"},
		{"2.1.299", "latest"},
		{"2.1.299", "^2.1.299"},
		{" validate --strict ", " validate "},
		{"- run:", "- if: false\n        run:"},
	} {
		changed := strings.Replace(source, mutation[0], mutation[1], 1)
		if changed == source {
			t.Fatal("plugin mutation did not change the fixture")
		}
		if inspectPlugin(agent, []byte(changed)) == "" {
			t.Fatal("incomplete plugin job accepted")
		}
	}
	for _, version := range []string{
		"2.1.295", "2.1.299", "2.1.300", "2.2.0", "2.10.0", "3.0.0",
	} {
		changed := strings.Replace(source, "2.1.299", version, 1)
		if reason := inspectPlugin(agent, []byte(changed)); reason != "" {
			t.Fatalf("supported exact version %s rejected: %s", version, reason)
		}
	}
	requireText(t, ".github/workflows/ci.yml",
		"go test -tags=network,releaseprobe ./tools/cicheck",
		"github.com/rhysd/actionlint/cmd/actionlint@v1.7.12",
		"github.com/goreleaser/goreleaser/v2@v2.12.7 check",
	)
}
