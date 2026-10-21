package policyfiles_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestGeneratedFreshMeasuresTargetExistence(t *testing.T) {
	t.Parallel()
	const root = "Makefile"
	const target = "generate-check"
	for _, tc := range []struct {
		name  string
		files map[string]string
		caps  []string
		want  bool
		limit bool
	}{
		{"near missing root, named by policy.makefile", nil, []string{"go"}, false, false},
		{"red missing target", map[string]string{root: "test:\n"}, []string{"go"}, true, false},
		{"red phony only", map[string]string{root: ".PHONY: generate-check\n"}, []string{"go"}, true, false},
		{"near literal", map[string]string{root: target + ":\n"}, []string{"go"}, false, false},
		{"near unrelated computed rule", map[string]string{root: target + ":\n$(BIN): $(SRC)\n"}, []string{"go"}, false, false},
		{"red matching pattern", map[string]string{root: "%-check:\n"}, []string{"go"}, false, true},
		{"red nonmatching pattern", map[string]string{root: "%.o:\n"}, []string{"go"}, true, false},
		{"red unrelated raw prerequisite", map[string]string{root: "build: $(SRC)\n"}, []string{"go"}, true, false},
		{"near modular", map[string]string{root: "include mk/generated.mk\n", "mk/generated.mk": target + ":\n"}, []string{"go"}, false, false},
		{"near no go", nil, nil, false, false},
		{"red unsupported target", map[string]string{root: "$(TARGET):\n"}, []string{"go"}, false, true},
		{"red unresolved required include", map[string]string{root: "include generated.mk\n"}, []string{"go"}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := measure(t, policyfiles.GeneratedFresh(), fixture{
				files: tc.files, capabilities: tc.caps,
			})
			if model.IsOutsideSubset(got.Status) != tc.limit {
				t.Fatalf("status = %+v", got.Status)
			}
			if tc.limit && len(got.Findings) != 0 {
				t.Fatalf("limit produced findings: %+v", got.Findings)
			}
			if slices.Contains(codes(got), "generate-check-missing") != tc.want {
				t.Fatalf("findings = %+v", got.Findings)
			}
			if tc.caps == nil && got.Status.Kind != model.StatusNotApplicable {
				t.Fatalf("capability gate = %+v", got.Status)
			}
		})
	}
}

func TestGeneratedFreshKeepsLimitsRelevant(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body, limit string
		missing           bool
	}{
		{"literal expansion", "CHECK := generate-check\n$(CHECK):\n", "", false},
		{"opaque recipe", "generate-check:\n\t./scripts/generate.sh || true\n", "", false},
		{"present beside missing include", "include absent.mk\ngenerate-check:\n", "", false},
		{"optional absent include", "-include absent.mk\n", "", true},
		{"conditional target", "ifdef MODE\ngenerate-check:\nendif\n", "Makefile:2:", false},
		{"explicit beside conditional", "generate-check:\nifdef MODE\ngenerate-check:\nendif\n", "", false},
		{"unrelated conditional", "ifdef MODE\nbuild:\nendif\n", "", true},
		{"conditional include", "ifdef MODE\ninclude absent.mk\nendif\n", "Makefile:2:", false},
		{"present beside conditional include", "generate-check:\nifdef MODE\ninclude absent.mk\nendif\n", "", false},
		{"unsupported double colon", "generate-check::\n", "Makefile:1:", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := measure(t, policyfiles.GeneratedFresh(), fixture{
				files: map[string]string{"Makefile": tc.body}, capabilities: []string{"go"},
			})
			requireGeneratedFreshEvidence(t, got, tc.limit, tc.missing)
		})
	}
}

func TestGeneratedFreshReadFailuresKeepTheActualPath(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"Makefile", "mk/generated.mk"} {
		for _, failure := range []error{model.ErrTooLarge, model.ErrUnreadable} {
			t.Run(path+"/"+failure.Error(), func(t *testing.T) {
				t.Parallel()
				policy := model.CoreDefaults()
				policy.Capabilities = []string{"go"}
				tree := proof.NewTree(map[string][]byte{
					"Makefile":        []byte("include mk/generated.mk\n"),
					"mk/generated.mk": []byte("generate-check:\n"),
				})
				got, err := policyfiles.GeneratedFresh().Run(t.Context(), model.Input{
					Policy: &policy, Snapshot: buildReadFailure{Snapshot: tree, path: path, err: failure},
				})
				if err != nil || len(got.Findings) != 0 {
					t.Fatalf("failed definition read = %+v, %v", got, err)
				}
				requireMakefileReadFailure(t, got, path, failure)
			})
		}
	}
}

func requireGeneratedFreshEvidence(t *testing.T, got model.Result, limit string, missing bool) {
	t.Helper()
	if limit != "" {
		if !model.IsOutsideSubset(got.Status) || len(got.Findings) != 0 || !strings.Contains(got.Status.Reason, limit) {
			t.Fatalf("target definition limit = %+v", got)
		}
		return
	}
	if got.Status.Kind != model.StatusMeasured || slices.Contains(codes(got), "generate-check-missing") != missing {
		t.Fatalf("definition evidence = %+v", got)
	}
	if missing {
		f := got.Findings[0]
		if len(got.Findings) != 1 || f.Path != "Makefile" || f.Line != 0 ||
			!strings.Contains(f.Message, "generate-check") || !strings.Contains(f.Message, "included file") {
			t.Fatalf("target repair = %+v", got)
		}
	}
}
