package cicheck_test

import (
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

const goldenRecipe = "\ngolden-gen:\n\tgo test -count=1 -v -run TestGolden ./internal/gen/...\n"

// goldenKinds are the kind packages of phase M7.1 a command creates.
func goldenKinds() []string {
	return []string{"adapterhttp", "adaptersql", "handler", "port", "usecase"}
}

// goldenTarget names what a Makefile lacks for the golden-gen target.
func goldenTarget(makefile string) []string {
	var missing []string
	if !strings.Contains(makefile, goldenRecipe) {
		missing = append(missing, "the golden-gen recipe")
	}
	ci, phony := "", ""
	for line := range strings.SplitSeq(makefile, "\n") {
		if rest, ok := strings.CutPrefix(line, "ci:"); ok {
			ci = rest
		}
		if rest, ok := strings.CutPrefix(line, ".PHONY:"); ok {
			phony = rest
		}
	}
	if !slices.Contains(strings.Fields(ci), "golden-gen") {
		missing = append(missing, "golden-gen in make ci")
	}
	if !slices.Contains(strings.Fields(phony), "golden-gen") {
		missing = append(missing, "golden-gen in .PHONY")
	}
	return missing
}

// kindProofs names what a kind package lacks: a golden slice test that runs
// the harness and the proof, its proof manifest and its recorded fills.
func kindProofs(tree fs.FS, kind string) []string {
	dir := path.Join("internal/gen/kinds", kind)
	var missing []string
	for _, file := range []string{"testdata/proof.yml", "testdata/fills.yml"} {
		if _, err := fs.Stat(tree, path.Join(dir, file)); err != nil {
			missing = append(missing, kind+": "+file)
		}
	}
	tests, err := fs.Glob(tree, path.Join(dir, "*_test.go"))
	if err != nil {
		return append(missing, kind+": "+err.Error())
	}
	golden, proved := false, false
	for _, name := range tests {
		data, readErr := fs.ReadFile(tree, name)
		text := string(data)
		golden = golden || readErr == nil && strings.Contains(text, "func TestGolden") && strings.Contains(text, "goldentest.Run(")
		proved = proved || readErr == nil && strings.Contains(text, "goldentest.Prove(")
	}
	if !golden {
		missing = append(missing, kind+": a TestGolden test calling goldentest.Run")
	}
	if !proved {
		missing = append(missing, kind+": a test calling goldentest.Prove")
	}
	return missing
}

// ciTools are the install lines the ci job needs so the golden slices and
// the fidelity tests run with the generators M7.10.3 names.
var ciTools = []string{
	"go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0\n",
	"go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1\n",
	"go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0\n",
}

func TestTheCIJobInstallsTheGoldenTools(t *testing.T) {
	t.Parallel()
	requireText(t, ".github/workflows/ci.yml", ciTools...)
}

func TestGoldenGenIsATargetOfMakeCI(t *testing.T) {
	t.Parallel()
	near := ".PHONY: mod ci golden-gen\n\nci: mod golden-gen\n" + goldenRecipe
	tests := map[string]struct {
		makefile string
		missing  int
	}{
		"red: no target":           {makefile: ".PHONY: mod ci\n\nci: mod\n", missing: 3},
		"red: a target outside ci": {makefile: ".PHONY: mod ci golden-gen\n\nci: mod\n" + goldenRecipe, missing: 1},
		"near: the target in ci":   {makefile: near},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := goldenTarget(tc.makefile); len(got) != tc.missing {
				t.Fatalf("goldenTarget = %q, want %d missing", got, tc.missing)
			}
		})
	}
	if got := goldenTarget(string(kitBytes(t, "Makefile"))); len(got) != 0 {
		t.Errorf("the kit's Makefile lacks %q", got)
	}
	if !strings.Contains(string(kitBytes(t, "Makefile")), "ci: mod deps-age ") {
		t.Error("make ci no longer starts with mod deps-age")
	}
}

func TestEveryKindCarriesItsGoldenProof(t *testing.T) {
	t.Parallel()
	golden := []byte("func TestGoldenPort(t *testing.T) { goldentest.Run(t, s); goldentest.Prove(t, p, l) }\n")
	tests := map[string]struct {
		tree    fstest.MapFS
		missing int
	}{
		"red: no proof manifest": {tree: fstest.MapFS{
			"internal/gen/kinds/port/golden_test.go":     {Data: golden},
			"internal/gen/kinds/port/testdata/fills.yml": {Data: []byte("fills: []\n")},
		}, missing: 1},
		"red: a golden test and no test of the proof": {tree: fstest.MapFS{
			"internal/gen/kinds/port/golden_test.go":     {Data: []byte("func TestGoldenPort(t *testing.T) { goldentest.Run(t, s) }\n")},
			"internal/gen/kinds/port/testdata/proof.yml": {Data: []byte("entries: []\n")},
			"internal/gen/kinds/port/testdata/fills.yml": {Data: []byte("fills: []\n")},
		}, missing: 1},
		"near: the complete proof": {tree: fstest.MapFS{
			"internal/gen/kinds/port/golden_test.go":     {Data: golden},
			"internal/gen/kinds/port/testdata/proof.yml": {Data: []byte("entries: []\n")},
			"internal/gen/kinds/port/testdata/fills.yml": {Data: []byte("fills: []\n")},
		}},
		"near: the proof in another test file of the kind": {tree: fstest.MapFS{
			"internal/gen/kinds/port/golden_test.go":     {Data: []byte("func TestGoldenSlicePort(t *testing.T) { goldentest.Run(t, s) }\n")},
			"internal/gen/kinds/port/proof_test.go":      {Data: []byte("func TestPortProof(t *testing.T) { goldentest.Prove(t, p, l) }\n")},
			"internal/gen/kinds/port/testdata/proof.yml": {Data: []byte("entries: []\n")},
			"internal/gen/kinds/port/testdata/fills.yml": {Data: []byte("fills: []\n")},
		}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := kindProofs(tc.tree, "port"); len(got) != tc.missing {
				t.Fatalf("kindProofs = %q, want %d missing", got, tc.missing)
			}
		})
	}
	kit := os.DirFS("../..")
	for _, kind := range goldenKinds() {
		if got := kindProofs(kit, kind); len(got) != 0 {
			t.Errorf("the kit lacks %q", got)
		}
	}
}
