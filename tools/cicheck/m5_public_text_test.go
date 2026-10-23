package cicheck_test

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/profiletest"
	"github.com/cristianargotti/golaw/internal/runner"
)

// m5FileKeys are the goreleaser keys that pack files from disk beside the
// binaries, so a release that carries one packs more than this test reads.
var m5FileKeys = []string{"files", "extra_files", "templated_files", "templated_extra_files"}

// m5SourcePattern is the source a stamp or a pin names, in block or flow form.
var m5SourcePattern = regexp.MustCompile(`\bsource:\s*["']?([^\s"',}]+)`)

// m5PackedExtras answers where a release configuration packs anything but
// the bare binaries: no archive at all or an archive in another format
// (each packs the README and the license by default), and every key that
// names files from disk.
func m5PackedExtras(config []byte) ([]string, error) {
	root, err := document(config)
	if err != nil {
		return nil, err
	}
	archives := children(at(root, "archives"))
	var extras []string
	if len(archives) == 0 {
		extras = append(extras, "archives")
	}
	for index, archive := range archives {
		if !slices.Equal(scalarList(at(archive, "formats")), []string{"binary"}) {
			extras = append(extras, fmt.Sprintf("archives.%d.formats", index))
		}
	}
	return append(extras, m5KeysNamingFiles(root, "")...), nil
}

// m5KeysNamingFiles answers the dotted path of every key under a node that
// names files from disk.
func m5KeysNamingFiles(node *yaml.Node, prefix string) []string {
	var found []string
	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i].Value
			if slices.Contains(m5FileKeys, key) {
				found = append(found, prefix+key)
			}
			found = append(found, m5KeysNamingFiles(node.Content[i+1], prefix+key+".")...)
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			found = append(found, m5KeysNamingFiles(child, prefix+strconv.Itoa(i)+".")...)
		}
	case yaml.DocumentNode, yaml.ScalarNode, yaml.AliasNode:
	}
	return found
}

// m5BuildArgs answers the go build arguments and the environment of the
// one build of the release configuration, so the test builds the binary
// goreleaser packs.
func m5BuildArgs(config []byte, output string) ([]string, []string, error) {
	root, err := document(config)
	if err != nil {
		return nil, nil, err
	}
	builds := children(at(root, "builds"))
	if len(builds) != 1 {
		return nil, nil, fmt.Errorf("%w: the release configures %d builds, want one", model.ErrUsage, len(builds))
	}
	main := value(at(builds[0], "main"))
	if main == "" {
		return nil, nil, fmt.Errorf("%w: the release build names no main package", model.ErrUsage)
	}
	args := append([]string{"build"}, scalarList(at(builds[0], "flags"))...)
	if ldflags := scalarList(at(builds[0], "ldflags")); len(ldflags) > 0 {
		args = append(args, "-ldflags="+strings.Join(ldflags, " "))
	}
	return append(args, "-o", output, main), scalarList(at(builds[0], "env")), nil
}

// m5Values are the values the render tests of the kit use, with a module path.
func m5Values(module string, capabilities []string) assets.Values {
	tools := make(map[string]string)
	for _, tool := range runner.Toolchain() {
		tools[tool.Name] = tool.Version
	}
	return assets.Values{
		ModulePath: module, GoVersion: "1.27", KitVersion: "0.2.0", Language: "pt-BR",
		Decisions: "docs/adr", Main: "root", Capabilities: capabilities, Tools: tools,
	}
}

// m5RenderedViolations renders every template under every capability set
// of the M3 render tests, and the service fixture, with a module path, and
// answers one line per rendered file the inspector refuses.
func m5RenderedViolations(module string, inspect func(string, []byte) string) ([]string, error) {
	var violations []string
	for _, capabilities := range m3Capabilities() {
		values := m5Values(module, capabilities)
		rendered, err := assets.RenderAllContext(golaw.FS, assets.Context{Values: values, CIBackend: "none"})
		if err != nil {
			return nil, fmt.Errorf("render the templates for %v: %w", capabilities, err)
		}
		service, err := assets.RenderService(golaw.FS, values)
		if err != nil {
			return nil, fmt.Errorf("render the service fixture: %w", err)
		}
		files := slices.Concat(rendered, service)
		for i := range files {
			if reason := inspect(files[i].Path, files[i].Data); reason != "" {
				violations = append(violations, strings.Join(capabilities, ",")+" "+files[i].Path+": "+reason)
			}
		}
	}
	return violations, nil
}

// m5ReservedHost reports whether a source names a host reserved for
// examples and tests, the only hosts a bundle of the public tree names.
func m5ReservedHost(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	for _, reserved := range []string{"test", "example", "invalid", "localhost", "example.com", "example.net", "example.org"} {
		if host == reserved || strings.HasSuffix(host, "."+reserved) {
			return true
		}
	}
	return false
}

func TestPackedExtrasNameWhatTheReleasePacksBesideTheBinary(t *testing.T) {
	t.Parallel()
	binary := "archives:\n  - id: binaries\n    ids: [golaw]\n    formats: [binary]\n"
	for name, test := range map[string]struct {
		config string
		want   []string
	}{
		"the bare binary":            {binary + "sboms:\n  - documents: [x.json]\nrelease:\n  draft: true\n", nil},
		"no archive":                 {"version: 2\n", []string{"archives"}},
		"a tarball":                  {"archives:\n  - formats: [tar.gz]\n", []string{"archives.0.formats"}},
		"files of an archive":        {"archives:\n  - formats: [binary]\n    files: [README.md]\n", []string{"archives.0.files"}},
		"extra files of the release": {binary + "release:\n  extra_files:\n    - glob: notes.txt\n", []string{"release.extra_files"}},
		"extra files of a checksum":  {binary + "checksum:\n  extra_files: [{glob: x}]\n", []string{"checksum.extra_files"}},
	} {
		got, err := m5PackedExtras([]byte(test.config))
		if err != nil || !slices.Equal(got, test.want) {
			t.Errorf("%s: %q, %v, want %q", name, got, err, test.want)
		}
	}
}

func TestBuildArgsFollowTheReleaseBuild(t *testing.T) {
	t.Parallel()
	args, env, err := m5BuildArgs(kitBytes(t, ".goreleaser.yml"), "out")
	want := []string{"build", "-trimpath", "-ldflags=-s -w", "-o", "out", "./cmd/golaw"}
	if err != nil || !slices.Equal(args, want) || !slices.Equal(env, []string{"CGO_ENABLED=0"}) {
		t.Errorf("m5BuildArgs = %q, %q, %v, want %q", args, env, err, want)
	}
	for name, config := range map[string]string{
		"two builds":    "builds:\n  - main: ./a\n  - main: ./b\n",
		"no build":      "version: 2\n",
		"no main named": "builds:\n  - id: golaw\n",
	} {
		if _, _, err := m5BuildArgs([]byte(config), "out"); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestReleasePacksABinaryThatNamesNoPrivateOrganization(t *testing.T) {
	t.Parallel()
	config := kitBytes(t, ".goreleaser.yml")
	if extras, err := m5PackedExtras(config); err != nil || len(extras) > 0 {
		t.Errorf("the release packs more than the binaries: %q, %v", extras, err)
	}
	output := filepath.Join(t.TempDir(), "golaw")
	args, env, err := m5BuildArgs(config, output)
	if err != nil {
		t.Fatal(err)
	}
	build := exec.CommandContext(t.Context(), "go", args...)
	build.Dir = filepath.Join("..", "..")
	// The release-config job runs go mod download first, so every module this
	// build needs is in the cache and the proxy stays off.
	build.Env = append(append(os.Environ(), env...), "GOFLAGS=-mod=readonly", "GOPROXY=off")
	if out, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("go %s: %v: %s", strings.Join(args, " "), buildErr, out)
	}
	binary, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if reason := newPublicTextInspector([]string{privateNameDigest})("golaw", binary); reason != "" {
		t.Errorf("the release binary: %s", reason)
	}
}

func TestRenderedTemplatesNameNoPrivateOrganization(t *testing.T) {
	t.Parallel()
	placeholder := newPublicTextInspector([]string{nameDigest(neutralName)})
	if got, err := m5RenderedViolations("example.test/"+neutralName, placeholder); err != nil || len(got) == 0 {
		t.Errorf("a module path that names the placeholder reaches the rendered files: %d, %v", len(got), err)
	}
	if got, err := m5RenderedViolations("example.test/"+neutralName+"s", placeholder); err != nil || len(got) > 0 {
		t.Errorf("another word is no name: %q, %v", got, err)
	}
	got, err := m5RenderedViolations("example.test/service", newPublicTextInspector([]string{privateNameDigest}))
	if err != nil || len(got) > 0 {
		t.Errorf("rendered templates: %q, %v", got, err)
	}
}

func TestTrackedBundlesAreSynthetic(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]bool{
		"https://example.test/profiles/fixture":                               true,
		"https://sub.example.test/x":                                          true,
		"https://docs.example.org/x":                                          true,
		"http://localhost:8080/x":                                             true,
		"https://github.com/" + neutralName + "/golaw-profile-" + neutralName: false,
		"https://example.testing.com/x":                                       false,
		"https://notexample.com/x":                                            false,
		"git@github.com:" + neutralName + "/x.git":                            false,
		"": false,
	} {
		if m5ReservedHost(raw) != want {
			t.Errorf("%q: reserved %v", raw, !want)
		}
	}
	ref := profiletest.Ref(t)
	if ref.Name != profiletest.Name || !m5ReservedHost(ref.Source) {
		t.Errorf("the fixture bundle is %s from %s, want %s from a reserved host", ref.Name, ref.Source, profiletest.Name)
	}
	tracked := trackedPaths(t)
	if !slices.Contains(tracked, "internal/profile/testdata/fixture/profile.yml") {
		t.Error("the synthetic fixture bundle is not tracked")
	}
	for _, rel := range tracked {
		if base := path.Base(rel); base != "profile.yml" && base != "golaw.yml" {
			continue
		}
		for _, match := range m5SourcePattern.FindAllSubmatch(kitBytes(t, rel), -1) {
			if !m5ReservedHost(string(match[1])) {
				t.Errorf("%s names the bundle source %s; the public tree holds synthetic bundles only", rel, match[1])
			}
		}
	}
}

func TestSourcePatternReadsTheStampAndThePin(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]string{
		"profile:\n  source: https://example.test/p\n":                                     "https://example.test/p",
		"profile: {name: x, source: https://github.com/" + neutralName + "/x, version: 1}": "https://github.com/" + neutralName + "/x",
		"profile: {source: https://example.test/q}":                                        "https://example.test/q",
		`profile: {source: 'git@github.com:` + neutralName + `/x.git'}`:                    "git@github.com:" + neutralName + "/x.git",
		"resource: https://github.com/x\n":                                                 "",
		"sources: https://github.com/x\n":                                                  "",
	} {
		got := ""
		if match := m5SourcePattern.FindStringSubmatch(text); match != nil {
			got = match[1]
		}
		if got != want {
			t.Errorf("%q: source %q, want %q", text, got, want)
		}
	}
}
