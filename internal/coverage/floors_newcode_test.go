package coverage_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/coverage"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestFloorsCarryTheNewCodeThreshold(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		in   model.Options
		want int
	}{
		{"near-no-policy-is-the-sonar-way-value", nil, model.DefaultCoverageNew},
		{"near-other-floors-leave-it", model.Options{"total": model.IntValue(95)}, model.DefaultCoverageNew},
		{"red-the-policy-raises-it", model.Options{"new_code": model.IntValue(95)}, 95},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := coverage.FloorsFrom(tc.in).NewCode; got != tc.want {
				t.Fatalf("NewCode = %d, want %d", got, tc.want)
			}
		})
	}
}
