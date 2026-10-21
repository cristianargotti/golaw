package policyfiles_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestMakefileIncludesMayBeRemadeByUnresolvedRules(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, extra string
		limited     bool
	}{
		{"near pattern remake", "%.mk:\n\tgenerate\n", true},
		{"near computed remake", "$(OUTPUT):\n\tgenerate\n", true},
		{"near conditional remake", "ifdef EXTRA\ngenerated.mk:\n\tgenerate\nendif\n", true},
		{"near top-level rules", "$(eval $(RULES))\n", true},
		{"near literal alias remake", "./generated.mk:\n\tgenerate\n", true},
		{"red nonmatching pattern", "source%.mk:\n\tgenerate\n", false},
		{"red nonmatching computed prefix", "src/$(OUTPUT):\n\tgenerate\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := measure(t, policyfiles.Makefile(), fixture{
				capabilities: []string{"go"},
				files:        map[string]string{"Makefile": "include generated.mk\n" + tc.extra},
			})
			if model.IsOutsideSubset(got.Status) != tc.limited {
				t.Fatalf("remade include = %+v", got)
			}
			if tc.limited {
				if len(got.Findings) != 0 || !strings.Contains(got.Status.Reason, "Makefile:") {
					t.Fatalf("include limit = %+v", got)
				}
			} else if !slices.Equal(codes(got), []string{"make-include-missing"}) {
				t.Fatalf("missing include = %+v", got)
			}
		})
	}
}

func TestMakefileIncludePatternsMatchTheBasename(t *testing.T) {
	t.Parallel()
	got := measure(t, policyfiles.Makefile(), fixture{
		capabilities: []string{"go"},
		files: map[string]string{
			"Makefile": "include mk/checks.mk\ncheck%.mk:; generate\n",
		},
	})
	if !model.IsOutsideSubset(got.Status) || len(got.Findings) != 0 ||
		!strings.Contains(got.Status.Reason, "check%.mk") {
		t.Fatalf("basename remake = %+v", got)
	}
}
