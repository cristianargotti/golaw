package toolchain

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
)

// minimal is the smallest table decodeTable accepts; each refusal below
// changes one line of it.
const minimal = `read: "2026-10-27"
runtimes:
  - {name: node, floor: "22.13.0", probe: [--version]}
  - {name: python, floor: "3.10", below: "3.15", probe: [--version]}
tools:
  - name: ruff
    version: "0.17.1"
    published: "2026-10-15"
    distribution: binary
    languages: [python]
    hook: true
    probe: [--version]
` + rufAssets + `  - name: yamllint
    version: "1.38.0"
    published: "2026-01-13"
    distribution: pypi
    languages: [yaml]
    probe: [--version]
    bin: yamllint
`

const sixtyFourA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// rufAssets is the asset block of the minimal ruff entry.
const rufAssets = "    assets:\n      - {platform: linux-amd64, url: \"https://github.com/x/ruff/0.17.1/ruff.tar.gz\", sha256: \"" +
	sixtyFourA + "\", archive: tar.gz, member: ruff/ruff}\n"

func TestDecodeTableAcceptsTheMinimalTable(t *testing.T) {
	t.Parallel()
	file, err := decodeTable([]byte(minimal))
	if err != nil {
		t.Fatalf("decodeTable = %v", err)
	}
	tools := file.tools()
	if len(tools) != 2 || tools[0].Name != "ruff" || tools[1].Bin != "yamllint" || tools[0].Assets[0].Member != "ruff/ruff" || tools[1].Script {
		t.Errorf("tools = %+v", tools)
	}
	script, err := decodeTable([]byte(strings.Replace(minimal, "    bin: yamllint\n", "    bin: yamllint\n    script: true\n", 1)))
	if err != nil || !script.tools()[1].Script {
		t.Errorf("a PyPI tool that ships a plain script = %v, %v", script, err)
	}
}

func TestDecodeTableRefusesWhatTheResolverCouldNotTrust(t *testing.T) {
	t.Parallel()
	tests := map[string]struct{ old, new string }{
		"unknown key":                        {"    hook: true\n", "    hook: true\n    vendored: true\n"},
		"published inside the seven days":    {`published: "2026-10-15"`, `published: "2026-10-22"`},
		"unknown distribution":               {"distribution: pypi", "distribution: brew"},
		"unknown language":                   {"languages: [yaml]", "languages: [cobol]"},
		"hook on a non-binary tool":          {"    distribution: pypi\n", "    distribution: pypi\n    hook: true\n"},
		"short checksum":                     {sixtyFourA, "abc"},
		"plain http asset":                   {"https://github.com/x/", "http://github.com/x/"},
		"archive without member":             {", member: ruff/ruff}", "}"},
		"xz archive the kit cannot open":     {"archive: tar.gz", "archive: tar.xz"},
		"binary tool without asset":          {rufAssets, "    bin: ruff\n"},
		"pypi tool without executable":       {"    bin: yamllint\n", ""},
		"script on a binary tool":            {"    hook: true\n", "    hook: true\n    script: true\n"},
		"repeated tool":                      {"  - name: yamllint", "  - name: ruff"},
		"version that is not semantic":       {`version: "1.38.0"`, `version: "latest"`},
		"runtime floor that is not semantic": {`floor: "3.10"`, `floor: "three"`},
		"runtime bound at its floor":         {`below: "3.15"`, `below: "3.10"`},
		"runtime bound that is not semantic": {`below: "3.15"`, `below: "next"`},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			changed := strings.Replace(minimal, tc.old, tc.new, 1)
			if changed == minimal {
				t.Fatal("the mutation did not change the table")
			}
			if _, err := decodeTable([]byte(changed)); !errors.Is(err, ErrTable) {
				t.Errorf("decodeTable = %v, want ErrTable", err)
			}
		})
	}
}

func TestPackageJSONPinsExactVersionsMatchingTheTable(t *testing.T) {
	t.Parallel()
	data, err := embeddedFile("package.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Private      bool              `json:"private"`
		Engines      map[string]string `json:"engines"`
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	exact := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	for name, version := range manifest.Dependencies {
		if !exact.MatchString(version) {
			t.Errorf("%s pins %q, want an exact version", name, version)
		}
	}
	for _, name := range []string{"eslint", "stylelint", "pyright"} {
		tool, _ := Lookup(name)
		if manifest.Dependencies[name] != tool.Version {
			t.Errorf("package.json pins %s %q, the table %q", name, manifest.Dependencies[name], tool.Version)
		}
	}
	if !manifest.Private || manifest.Engines["node"] != ">=22.13.0" || manifest.Dependencies["typescript"] != "6.0.3" {
		t.Errorf("manifest = %+v, want private, node >=22.13.0 and typescript 6.0.3", manifest)
	}
}

func TestPackageLockCarriesTheIntegrityOfEveryPackage(t *testing.T) {
	t.Parallel()
	data, err := embeddedFile("package-lock.json")
	if err != nil {
		t.Fatal(err)
	}
	var lock struct {
		LockfileVersion int `json:"lockfileVersion"`
		Packages        map[string]struct {
			Resolved  string `json:"resolved"`
			Integrity string `json:"integrity"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		t.Fatal(err)
	}
	if lock.LockfileVersion != 3 || len(lock.Packages) < 2 {
		t.Fatalf("lock version %d with %d packages", lock.LockfileVersion, len(lock.Packages))
	}
	for path, entry := range lock.Packages {
		if path == "" {
			continue
		}
		if !strings.HasPrefix(entry.Integrity, "sha512-") || !strings.HasPrefix(entry.Resolved, "https://registry.npmjs.org/") {
			t.Errorf("%s: integrity %q from %q", path, entry.Integrity, entry.Resolved)
		}
	}
}

func TestRequirementsLockHashesEveryDistribution(t *testing.T) {
	t.Parallel()
	data, err := embeddedFile("requirements.lock")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.ReplaceAll(string(data), "\\\n", " ")
	requirement := regexp.MustCompile(`^([A-Za-z0-9_.-]+)==([0-9][^ ;]*)(?: *;.*?)?((?: +--hash=sha256:[0-9a-f]{64})+)\s*$`)
	found := make(map[string]string)
	for line := range strings.SplitSeq(joined, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		match := requirement.FindStringSubmatch(line)
		if match == nil {
			t.Errorf("requirement without a pin and its hashes: %q", line)
			continue
		}
		found[strings.ToLower(match[1])] = match[2]
	}
	for name, version := range map[string]string{"cfn-lint": "1.58.0", "checkov": "3.3.30", "yamllint": "1.38.0"} {
		if found[name] != version {
			t.Errorf("requirements.lock pins %s %q, want %s", name, found[name], version)
		}
	}
}

func TestLockIDNamesEachLockApart(t *testing.T) {
	t.Parallel()
	npm, pypi := lockID("package-lock.json"), lockID("requirements.lock")
	hex := regexp.MustCompile(`^[0-9a-f]{16}$`)
	if !hex.MatchString(npm) || !hex.MatchString(pypi) || npm == pypi {
		t.Errorf("lock ids %q and %q, want two distinct 16 digit ids", npm, pypi)
	}
}
