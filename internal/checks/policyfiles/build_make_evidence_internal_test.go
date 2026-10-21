package policyfiles

import "testing"

func TestMakeTargetEvidenceKeepsLiteralAnchors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, token, target string
		want                bool
	}{
		{"near literal target", "lint", "lint", false},
		{"near unknown prefix", "$(PREFIX)int", "lint", true},
		{"near unknown suffix", "li${SUFFIX}", "lint", true},
		{"near unknown interior", "l$(MIDDLE)t", "lint", true},
		{"near prefix mismatch", "build-$(PART)", "lint", false},
		{"near suffix mismatch", "$(PART)-check", "lint", false},
		{"near interior mismatch", "$(A)zz$(B)", "lint", false},
		{"near ordered interiors", "$(A)l$(B)n$(C)t", "lint", true},
		{"near reversed interiors", "$(A)n$(B)l$(C)t", "lint", false},
		{"near nonoverlapping anchors", "lin$(PART)int", "lint", false},
		{"near nested expression", "l$(if ${MODE},i,$(OTHER))nt", "lint", true},
		{"near short reference", "$T", "lint", true},
		{"near short interior reference", "l$Tt", "lint", true},
		{"near unicode short reference", "l$\u754ct", "lint", true},
		{"near escaped dollar", "$$lint", "lint", false},
		{"near trailing dollar", "lint$", "lint", true},
		{"near empty variable span", "li$(PART)nt", "lint", true},
		{"near matching pattern", "l%t", "lint", true},
		{"near basename pattern", "check%.mk", "mk/checks.mk", true},
		{"near directory pattern", "mk/check%.mk", "mk/checks.mk", true},
		{"near other directory", "mk/check%.mk", "src/checks.mk", false},
		{"near basename empty stem", "lint%", "mk/lint", false},
		{"near computed target keeps directory", "check$(NAME).mk", "mk/checks.mk", false},
		{"near mismatching pattern", "%-check", "lint", false},
		{"near empty pattern prefix", "%lint", "lint", false},
		{"near empty pattern suffix", "lint%", "lint", false},
		{"near empty pattern interior", "li%nt", "lint", false},
		{"near variable before empty pattern", "li$(PART)%nt", "lint", false},
		{"near variable after empty pattern", "li%$(PART)nt", "lint", false},
		{"near empty pattern before empty variable", "lint%$(PART)", "lint", false},
		{"near pattern between variables", "$(LEFT)%$(RIGHT)", "lint", true},
		{"near second percent literal", "l%i%t", "lxi%t", true},
		{"near empty pattern beside literal percent", "l%i%t", "li%t", false},
		{"near second percent cannot expand", "l%i%t", "lint", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := makeUnknownTarget(tc.token, tc.target); got != tc.want {
				t.Fatalf("target %q from %q: uncertain = %v, want %v", tc.target, tc.token, got, tc.want)
			}
		})
	}
}

func TestMakeTargetEvidenceNeedsAnAbsentTarget(t *testing.T) {
	t.Parallel()
	m := makefile{
		rules: []makeRule{
			{targets: []string{"$(TARGET)"}, file: "rules.mk", line: 1},
			{targets: []string{"lint"}, file: "rules.mk", line: 2, condition: "rules.mk:1"},
			{targets: []string{"lint"}, file: "Makefile", line: 1},
		},
		missing: []makeInclude{{name: "optional.mk", file: "Makefile", line: 2, condition: "Makefile:1"}},
		opaque:  []makeOpaque{{text: "$(eval $(RULE))", file: "Makefile", line: 3}},
	}
	if err := makeTargetLimit(m, "lint"); err != nil {
		t.Fatalf("existing target was limited: %v", err)
	}
}
