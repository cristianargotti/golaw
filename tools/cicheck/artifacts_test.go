package cicheck_test

import (
	"strings"
	"testing"
)

func TestReleaseArtifactConfiguration(t *testing.T) {
	t.Parallel()
	data := kitBytes(t, ".goreleaser.yml")
	if reason := inspectArtifacts(".goreleaser.yml", data); reason != "" {
		t.Fatal(reason)
	}
	for _, mutation := range [][2]string{
		{"goarch: [amd64, arm64]", "goarch: [amd64]"},
		{"artifacts: binary", "artifacts: archive"},
		{"ids: [binaries]", "ids: [golaw]"},
		{"--new-bundle-format", "--output-signature"},
		{"draft: true", "draft: false"},
	} {
		changed := strings.Replace(string(data), mutation[0], mutation[1], 1)
		if changed == string(data) {
			t.Fatal("artifact mutation did not change its target")
		}
		if inspectArtifacts(".goreleaser.yml", []byte(changed)) == "" {
			t.Fatal("incomplete artifact configuration accepted")
		}
	}
}
