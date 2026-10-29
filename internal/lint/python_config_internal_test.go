package lint

import (
	"encoding/json"
	"io/fs"
	"maps"
	"regexp"
	"slices"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/cristianargotti/golaw/internal/model"
)

// pythonRuffSettings is the part of ruff.toml the kit writes; a key the
// test does not know fails the decode, so the asset never drifts unseen.
type pythonRuffSettings struct {
	Lint pythonRuffLint `toml:"lint"`
}

type pythonRuffLint struct {
	Select         []string            `toml:"select"`
	Ignore         []string            `toml:"ignore"`
	PerFileIgnores map[string][]string `toml:"per-file-ignores"`
	Mccabe         pythonRuffMccabe    `toml:"mccabe"`
	Pylint         pythonRuffPylint    `toml:"pylint"`
}

type pythonRuffMccabe struct {
	MaxComplexity int `toml:"max-complexity"`
}

type pythonRuffPylint struct {
	MaxBranches int `toml:"max-branches"`
}

// pythonRuleCode is one ruff rule code; a prefix such as S or ALL in the
// parity set would enable rules no Sonar way key maps.
var pythonRuleCode = regexp.MustCompile(`^[A-Z]+\d+$`)

func pythonReadRuff(t *testing.T, set string) pythonRuffLint {
	t.Helper()
	data, err := fs.ReadFile(pythonConfig(), set+"/"+pythonRuffConfig)
	if err != nil {
		t.Fatalf("read %s/%s: %v", set, pythonRuffConfig, err)
	}
	var settings pythonRuffSettings
	meta, err := toml.Decode(string(data), &settings)
	if err != nil {
		t.Fatalf("decode %s/%s: %v", set, pythonRuffConfig, err)
	}
	if extra := meta.Undecoded(); len(extra) != 0 {
		t.Fatalf("%s/%s holds keys the test does not know: %v", set, pythonRuffConfig, extra)
	}
	return settings.Lint
}

func pythonReadPyright(t *testing.T, set string) map[string]string {
	t.Helper()
	data, err := fs.ReadFile(pythonConfig(), set+"/"+pythonPyrightConfig)
	if err != nil {
		t.Fatalf("read %s/%s: %v", set, pythonPyrightConfig, err)
	}
	var settings map[string]string
	if err = json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("decode %s/%s: %v", set, pythonPyrightConfig, err)
	}
	return settings
}

func pythonExpectLimits(t *testing.T, set string, cfg pythonRuffLint) {
	t.Helper()
	if cfg.Mccabe.MaxComplexity != 10 || cfg.Pylint.MaxBranches != 12 {
		t.Errorf("%s limits = complexity %d, branches %d; want 10 (C901 at 10) and 12 (PLR0912 at its default)",
			set, cfg.Mccabe.MaxComplexity, cfg.Pylint.MaxBranches)
	}
}

func TestPythonConfigTreeHoldsTwoFilesPerSet(t *testing.T) {
	t.Parallel()
	var got []string
	err := fs.WalkDir(pythonConfig(), ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr == nil && !d.IsDir() {
			got = append(got, path)
		}
		return walkErr
	})
	if err != nil {
		t.Fatalf("walk the python tree: %v", err)
	}
	want := []string{"parity/pyrightconfig.json", "parity/ruff.toml", "strict/pyrightconfig.json", "strict/ruff.toml"}
	if !slices.Equal(got, want) {
		t.Errorf("python tree = %v, want %v", got, want)
	}
}

func TestPythonParitySelectsTheHubCodes(t *testing.T) {
	t.Parallel()
	cfg := pythonReadRuff(t, model.SurfaceParity)
	tests := map[string]struct {
		code     string
		selected bool
	}{
		"S105 password string (python:S2068)":           {code: "S105", selected: true},
		"S106 password argument (python:S2068)":         {code: "S106", selected: true},
		"S107 password default (python:S2068)":          {code: "S107", selected: true},
		"S307 eval (python:S1523)":                      {code: "S307", selected: true},
		"S102 exec (python:S1523)":                      {code: "S102", selected: true},
		"C420 dict.fromkeys shape (python:S7519)":       {code: "C420", selected: true},
		"C901 complexity (python:S3776)":                {code: "C901", selected: true},
		"PLR0912 branches (python:S3776)":               {code: "PLR0912", selected: true},
		"BLE001 blind except (python:S5754)":            {code: "BLE001", selected: true},
		"E722 bare except (python:S5754)":               {code: "E722", selected: true},
		"S101 assert (near miss: no Sonar way key)":     {code: "S101", selected: false},
		"E501 line length (near miss: style only)":      {code: "E501", selected: false},
		"D100 module docstring (near miss: style only)": {code: "D100", selected: false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := slices.Contains(cfg.Select, tc.code); got != tc.selected {
				t.Errorf("parity selects %s = %v, want %v", tc.code, got, tc.selected)
			}
		})
	}
}

func TestPythonParitySelectsCodesOnly(t *testing.T) {
	t.Parallel()
	cfg := pythonReadRuff(t, model.SurfaceParity)
	if len(cfg.Select) == 0 || !slices.IsSorted(cfg.Select) || len(slices.Compact(slices.Clone(cfg.Select))) != len(cfg.Select) {
		t.Errorf("parity select = %v, want a sorted list without repeats", cfg.Select)
	}
	for _, code := range cfg.Select {
		if !pythonRuleCode.MatchString(code) {
			t.Errorf("parity selects %q: only whole rule codes keep the set equal to the rule map", code)
		}
	}
	if len(cfg.Ignore) != 0 || len(cfg.PerFileIgnores) != 0 {
		t.Errorf("parity ignores %v and %v, want none: a parity code is never silenced", cfg.Ignore, cfg.PerFileIgnores)
	}
	pythonExpectLimits(t, model.SurfaceParity, cfg)
}

func TestPythonStrictSelectsAllWithTheFormatterIgnores(t *testing.T) {
	t.Parallel()
	cfg := pythonReadRuff(t, model.SurfaceStrict)
	if !slices.Equal(cfg.Select, []string{"ALL"}) {
		t.Errorf("strict select = %v, want [ALL]", cfg.Select)
	}
	want := []string{
		"COM812", "COM819", "CPY", "D203", "D206", "D213", "D300", "E111",
		"E114", "E117", "ISC002", "Q000", "Q001", "Q002", "Q003", "Q004", "W191",
	}
	if !slices.Equal(cfg.Ignore, want) {
		t.Errorf("strict ignore = %v, want %v", cfg.Ignore, want)
	}
	tests := map[string][]string{
		"test_*.py":   {"ANN", "D1", "PLR2004", "S101"},
		"*_test.py":   {"ANN", "D1", "PLR2004", "S101"},
		"conftest.py": {"ANN", "D1", "PLR2004", "S101"},
	}
	if !maps.EqualFunc(cfg.PerFileIgnores, tests, slices.Equal[[]string]) {
		t.Errorf("strict per-file-ignores = %v, want %v", cfg.PerFileIgnores, tests)
	}
	pythonExpectLimits(t, model.SurfaceStrict, cfg)
}

func TestPythonPyrightSets(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		set  string
		want map[string]string
	}{
		"parity runs the one rule a Sonar way key maps": {set: model.SurfaceParity, want: map[string]string{
			"typeCheckingMode":          "off",
			"reportAssignmentType":      "error",
			"reportMissingImports":      "none",
			"reportMissingModuleSource": "none",
		}},
		"strict runs the strict mode (near miss: never off)": {set: model.SurfaceStrict, want: map[string]string{
			"typeCheckingMode":          "strict",
			"reportMissingImports":      "none",
			"reportMissingModuleSource": "none",
			"reportUnknownArgumentType": "none",
			"reportUnknownMemberType":   "none",
			"reportUnknownVariableType": "none",
		}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := pythonReadPyright(t, tc.set); !maps.Equal(got, tc.want) {
				t.Errorf("%s/%s = %v, want %v", tc.set, pythonPyrightConfig, got, tc.want)
			}
		})
	}
}
