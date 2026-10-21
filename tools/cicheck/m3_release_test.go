package cicheck_test

import (
	"strings"
	"testing"
)

func TestM3RetainsExactReleaseEligibility(t *testing.T) {
	t.Parallel()
	const path = ".github/workflows/release.yml"
	source := string(kitBytes(t, path))
	for _, test := range []struct {
		name, old, next string
		fail            bool
	}{
		{"near", "needs: gate", "needs: gate", false},
		{"missing gate", "needs: gate", "needs: []", true},
		{"unconditional publication", "needs: gate", "needs: gate\n    if: always()", true},
		{"different checkout", "ref: ${{ needs.gate.outputs.sha }}", "ref: ${{ github.sha }}", true},
		{"eligibility omitted", "bash tools/cicheck/release-gate.sh", "true", true},
		{"acceptance weakened", "go run ./tools/acceptance -release", "go run ./tools/acceptance", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			changed := strings.Replace(source, test.old, test.next, 1)
			if test.old != test.next && changed == source {
				t.Fatal("release mutation did not change the source")
			}
			reason := inspectRelease(path, []byte(changed))
			if (reason != "") != test.fail {
				t.Fatalf("reason=%q, want failure=%t", reason, test.fail)
			}
		})
	}
}
