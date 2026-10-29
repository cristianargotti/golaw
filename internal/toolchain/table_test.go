package toolchain_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// pin is what the table holds for one tool (M6.4.2, M6.5.1), each the
// newest release at least seven days old when the pins were read.
type pin struct {
	version   string
	dist      toolchain.Distribution
	hook      bool
	languages []surface.Language
	assets    int
}

func pins() map[string]pin {
	return map[string]pin{
		"cfn-lint":   {"1.58.0", toolchain.DistPyPI, false, []surface.Language{surface.CloudFormation}, 0},
		"checkov":    {"3.3.30", toolchain.DistPyPI, false, []surface.Language{surface.Dockerfile, surface.CloudFormation}, 0},
		"eslint":     {"10.13.0", toolchain.DistNPM, false, []surface.Language{surface.JavaScript, surface.TypeScript, surface.JSX, surface.TSX, surface.HTML}, 0},
		"hadolint":   {"2.15.1", toolchain.DistBinary, true, []surface.Language{surface.Dockerfile}, 5},
		"pyright":    {"1.1.414", toolchain.DistNPM, false, []surface.Language{surface.Python}, 0},
		"ruff":       {"0.17.1", toolchain.DistBinary, true, []surface.Language{surface.Python}, 6},
		"shellcheck": {"0.11.0", toolchain.DistBinary, true, []surface.Language{surface.Shell}, 5},
		"shfmt":      {"3.14.1", toolchain.DistBinary, false, []surface.Language{surface.Shell}, 5},
		"stylelint":  {"17.16.0", toolchain.DistNPM, false, []surface.Language{surface.CSS, surface.HTML}, 0},
		"yamllint":   {"1.38.0", toolchain.DistPyPI, false, []surface.Language{surface.YAML}, 0},
	}
}

func table(t *testing.T) []toolchain.Tool {
	t.Helper()
	tools, err := toolchain.Table()
	if err != nil {
		t.Fatalf("Table() = %v", err)
	}
	return tools
}

func TestTableHoldsEveryPin(t *testing.T) {
	t.Parallel()
	want := pins()
	tools := table(t)
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
		p, ok := want[tool.Name]
		if !ok {
			t.Errorf("unexpected tool %q", tool.Name)
			continue
		}
		if tool.Version != p.version || tool.Distribution != p.dist || tool.Hook != p.hook || !slices.Equal(tool.Languages, p.languages) {
			t.Errorf("%s = %s %s hook=%t %v, want %+v", tool.Name, tool.Version, tool.Distribution, tool.Hook, tool.Languages, p)
		}
		if len(tool.Assets) != p.assets || len(tool.Probe) == 0 {
			t.Errorf("%s: %d assets and probe %v, want %d assets and a probe", tool.Name, len(tool.Assets), tool.Probe, p.assets)
		}
	}
	if len(names) != len(want) || !slices.IsSorted(names) {
		t.Errorf("tools = %v, want the %d pins sorted by name", names, len(want))
	}
}

func TestOnlyTheThreeFastToolsMayRunInTheHook(t *testing.T) {
	t.Parallel()
	var hook []string
	for _, tool := range table(t) {
		if tool.Hook {
			hook = append(hook, tool.Name)
		}
	}
	if want := []string{"hadolint", "ruff", "shellcheck"}; !slices.Equal(hook, want) {
		t.Errorf("hook tools = %v, want %v (A28)", hook, want)
	}
}

func TestEveryBinaryAssetIsVerifiableAndEveryOtherToolHasAnExecutable(t *testing.T) {
	t.Parallel()
	digest := regexp.MustCompile(`^[0-9a-f]{64}$`)
	platforms := []string{"darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64", "windows-amd64", "windows-arm64"}
	for _, tool := range table(t) {
		if tool.Distribution != toolchain.DistBinary {
			if tool.Bin == "" || len(tool.Assets) != 0 {
				t.Errorf("%s: bin %q and %d assets, want an executable name and no asset", tool.Name, tool.Bin, len(tool.Assets))
			}
			continue
		}
		seen := make(map[string]bool)
		for _, asset := range tool.Assets {
			switch {
			case !slices.Contains(platforms, asset.Platform) || seen[asset.Platform]:
				t.Errorf("%s: platform %q unknown or repeated", tool.Name, asset.Platform)
			case !digest.MatchString(asset.SHA256):
				t.Errorf("%s %s: sha256 %q is not 64 lower case hex digits", tool.Name, asset.Platform, asset.SHA256)
			case !strings.HasPrefix(asset.URL, "https://github.com/") || !strings.Contains(asset.URL, tool.Version):
				t.Errorf("%s %s: url %q is not a release asset of the pin", tool.Name, asset.Platform, asset.URL)
			case !slices.Contains([]string{"", "tar.gz", "zip"}, asset.Archive) || (asset.Archive == "") != (asset.Member == ""):
				t.Errorf("%s %s: archive %q with member %q", tool.Name, asset.Platform, asset.Archive, asset.Member)
			}
			seen[asset.Platform] = true
		}
	}
}

func TestLookupFindsAPinnedToolByItsName(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		name string
		want bool
	}{
		"a binary tool":                   {name: "ruff", want: true},
		"an npm tool":                     {name: "eslint", want: true},
		"a plugin is no tool (near miss)": {name: "eslint-plugin-react"},
		"empty name (near miss)":          {name: ""},
	}
	for label, tc := range tests {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			tool, ok := toolchain.Lookup(tc.name)
			if ok != tc.want || ok && tool.Name != tc.name {
				t.Errorf("Lookup(%q) = %+v, %t, want found=%t", tc.name, tool, ok, tc.want)
			}
		})
	}
}

func TestForLanguagesAnswersTheToolsAnyLanguageNeeds(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		langs []surface.Language
		want  []string
	}{
		"python":                       {[]surface.Language{surface.Python}, []string{"pyright", "ruff"}},
		"dockerfile":                   {[]surface.Language{surface.Dockerfile}, []string{"checkov", "hadolint"}},
		"cloudformation":               {[]surface.Language{surface.CloudFormation}, []string{"cfn-lint", "checkov"}},
		"html scripts and styles":      {[]surface.Language{surface.HTML}, []string{"eslint", "stylelint"}},
		"two languages, no repetition": {[]surface.Language{surface.JavaScript, surface.Python, surface.TSX}, []string{"eslint", "pyright", "ruff"}},
		"shell":                        {[]surface.Language{surface.Shell}, []string{"shellcheck", "shfmt"}},
		"yaml":                         {[]surface.Language{surface.YAML}, []string{"yamllint"}},
		"no language (near miss)":      {nil, nil},
		"unknown language (near miss)": {[]surface.Language{"cobol"}, nil},
	}
	for label, tc := range tests {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			tools := toolchain.ForLanguages(tc.langs)
			var got []string
			if tools != nil {
				got = make([]string, 0, len(tools))
			}
			for _, tool := range tools {
				got = append(got, tool.Name)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("ForLanguages(%v) = %v, want %v", tc.langs, got, tc.want)
			}
		})
	}
}

func TestEverySurfaceLanguageHasATool(t *testing.T) {
	t.Parallel()
	for _, language := range surface.Languages() {
		if len(toolchain.ForLanguages([]surface.Language{language})) == 0 {
			t.Errorf("no tool lints %s", language)
		}
	}
}

func TestRuntimesAreTheTwoPrerequisites(t *testing.T) {
	t.Parallel()
	got := toolchain.Runtimes()
	want := []toolchain.Runtime{
		{Name: "node", Floor: "22.13.0", Probe: []string{"--version"}},
		{Name: "python", Floor: "3.10", Below: "3.15", Probe: []string{"--version"}},
	}
	if len(got) != len(want) {
		t.Fatalf("Runtimes() = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i].Name != want[i].Name || got[i].Floor != want[i].Floor || got[i].Below != want[i].Below || !slices.Equal(got[i].Probe, want[i].Probe) {
			t.Errorf("runtime %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	for name, runtime := range map[string]string{"ruff": "", "eslint": "node", "pyright": "node", "yamllint": "python"} {
		if tool, _ := toolchain.Lookup(name); tool.Runtime() != runtime {
			t.Errorf("%s.Runtime() = %q, want %q", name, tool.Runtime(), runtime)
		}
	}
}

func TestDigestIsAStableSha256AndPlatformJoinsTheTarget(t *testing.T) {
	t.Parallel()
	first, second := toolchain.Digest(), toolchain.Digest()
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(first) || first != second {
		t.Errorf("Digest() = %q then %q, want one stable sha256", first, second)
	}
	for _, tc := range []struct{ goos, goarch, want string }{
		{"darwin", "arm64", "darwin-arm64"},
		{"windows", "amd64", "windows-amd64"},
	} {
		if got := toolchain.Platform(tc.goos, tc.goarch); got != tc.want {
			t.Errorf("Platform(%s, %s) = %q, want %q", tc.goos, tc.goarch, got, tc.want)
		}
	}
}
