package cicheck_test

import (
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/release"
)

// sampleVersion renders the name templates of the packing file, whose
// version goreleaser takes from the tag without its v.
const sampleVersion = "1.2.3"

// TestReleaseVerifierReadsWhatTheLaneWrites holds the names doctor looks up
// in a release directory and the identity it trusts to what the lane packs,
// signs and verifies, because a verifier that looks up another asset name or
// trusts another signer refuses every genuine release: a false finding on
// the first machine that runs it.
func TestReleaseVerifierReadsWhatTheLaneWrites(t *testing.T) {
	t.Parallel()
	packed := string(kitBytes(t, ".goreleaser.yml"))
	lane := string(kitBytes(t, ".github/workflows/release.yml"))
	if reason := inspectReleaseNames([]byte(packed), []byte(lane)); reason != "" {
		t.Fatal(reason)
	}
	for _, test := range []struct {
		packed, lane [2]string
		refused      bool
	}{
		{packed: [2]string{"name_template: 'golaw_{{ .Version }}_{{ .Os }}-{{ .Arch }}'", "name_template: 'golaw_{{ .Version }}_{{ .Os }}_{{ .Arch }}'"}, refused: true},
		{packed: [2]string{"formats: [binary]", "formats: [tar.gz]"}, refused: true},
		{packed: [2]string{"name_template: checksums.txt", "name_template: SHA256SUMS"}, refused: true},
		{packed: [2]string{"signature: '${artifact}.sigstore.json'", "signature: '${artifact}.sig'"}, refused: true},
		{packed: [2]string{"goos: [darwin, linux, windows]", "goos: []"}, refused: true},
		{lane: [2]string{"release.yml@${GITHUB_REF}", "release.yml@refs/heads/main"}, refused: true},
		{lane: [2]string{"https://token.actions.githubusercontent.com", "https://token.actions.githubusercontent.com.invalid"}, refused: true},
		{packed: [2]string{"formats: [binary]", "formats:\n      - binary"}},
		{packed: [2]string{"goarch: [amd64, arm64]", "goarch: [arm64, amd64]"}},
		{packed: [2]string{"goos: [darwin, linux, windows]", "goos: [darwin, freebsd, linux, windows]"}},
	} {
		changedPacked, changedLane := mutated(t, packed, test.packed), mutated(t, lane, test.lane)
		reason := inspectReleaseNames([]byte(changedPacked), []byte(changedLane))
		if (reason != "") != test.refused {
			t.Errorf("%v %v: reason %q, want refused %v", test.packed, test.lane, reason, test.refused)
		}
	}
}

// mutated applies one replacement and fails when it changes nothing, so a
// case never passes because its target moved.
func mutated(t *testing.T, text string, mutation [2]string) string {
	t.Helper()
	if mutation[0] == "" {
		return text
	}
	changed := strings.Replace(text, mutation[0], mutation[1], 1)
	if changed == text {
		t.Fatalf("mutation did not change its target: %q", mutation[0])
	}
	return changed
}

// inspectReleaseNames answers why the verifier would not find what the
// lane publishes, empty when every name and the identity agree.
func inspectReleaseNames(packed, lane []byte) string {
	if reason := inspectPackedNames(packed); reason != "" {
		return reason
	}
	root, err := document(lane)
	if err != nil {
		return "The release lane cannot be decoded."
	}
	verify := value(at(stepByID(at(root, "jobs", "release"), "verify"), "run"))
	id := release.Identity{Lane: release.Lane, Issuer: release.DefaultIssuer}
	tagRef := "refs/tags/v" + sampleVersion
	identity := strings.TrimSuffix(id.Subject(sampleVersion), tagRef) + "${GITHUB_REF}"
	if !strings.Contains(verify, "--certificate-identity \\\n  \""+identity+"\"") {
		return "The lane verifies another identity than release.Identity.Subject."
	}
	if !strings.Contains(verify, "--certificate-oidc-issuer \\\n  "+id.Issuer+" \\\n") {
		return "The lane verifies another issuer than release.DefaultIssuer."
	}
	if !strings.Contains(verify, "--bundle dist/"+release.ChecksumsFile+release.BundleSuffix) {
		return "The lane verifies another bundle than the one doctor reads."
	}
	return ""
}

// inspectPackedNames compares the asset, checksum and bundle names the
// packing file writes with the names release looks up.
func inspectPackedNames(packed []byte) string {
	root, err := document(packed)
	if err != nil {
		return "The packing file cannot be decoded."
	}
	archive := at(root, "archives", "0")
	if !slices.Equal(scalarList(at(archive, "formats")), []string{"binary"}) {
		return "The release assets must be the bare binaries release.AssetName names."
	}
	template := value(at(archive, "name_template"))
	goos, goarch := scalarList(at(root, "builds", "0", "goos")), scalarList(at(root, "builds", "0", "goarch"))
	if len(goos) == 0 || len(goarch) == 0 {
		return "The packing file names no platform to compare."
	}
	for _, system := range goos {
		for _, arch := range goarch {
			// A bare binary archive keeps the extension of the binary, .exe on windows.
			name := strings.NewReplacer("{{ .Version }}", sampleVersion, "{{ .Os }}", system, "{{ .Arch }}", arch).Replace(template)
			if system == "windows" {
				name += ".exe"
			}
			if name != release.AssetName(sampleVersion, system, arch) {
				return "The asset " + name + " is not release.AssetName of its platform."
			}
		}
	}
	if value(at(root, "checksum", "name_template")) != release.ChecksumsFile {
		return "The checksum file is not release.ChecksumsFile."
	}
	return inspectPackedBundle(root)
}

// inspectPackedBundle finds the signature of checksums.txt where doctor
// reads it: the checksum file with the bundle suffix beside it.
func inspectPackedBundle(root *yaml.Node) string {
	for _, sign := range children(at(root, "signs")) {
		if value(at(sign, "artifacts")) == "checksum" && value(at(sign, "signature")) == "${artifact}"+release.BundleSuffix {
			return ""
		}
	}
	return "The checksum bundle is not " + release.ChecksumsFile + release.BundleSuffix + "."
}
