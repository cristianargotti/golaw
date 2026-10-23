package profile_test

import (
	"fmt"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/profile"
)

// TestCauseDropsThePolicySentinels pins the text a person reads from a
// lifecycle or doctor line: the cause without the leading sentinels of the
// policy chain, while the error itself keeps them for errors.Is.
func TestCauseDropsThePolicySentinels(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		err  error
		want string
	}{
		"bundle": {
			fmt.Errorf("%w: %w: profile fixture has no earlier version to roll back to", model.ErrPolicy, profile.ErrBundle),
			"profile fixture has no earlier version to roll back to",
		},
		"policy": {
			fmt.Errorf("%w: profile fixture is not installed: run golaw profile install x@1.2.3", model.ErrPolicy),
			"profile fixture is not installed: run golaw profile install x@1.2.3",
		},
		"near digest": {
			fmt.Errorf("%w: %w: profile fixture: digest a differs from its stamp b", model.ErrPolicy, profile.ErrDigest),
			"profile digest mismatch: profile fixture: digest a differs from its stamp b",
		},
		"near inner sentinel text": {
			fmt.Errorf("resolve the state directories: %w: HOME is unset", model.ErrPolicy),
			"resolve the state directories: invalid policy: HOME is unset",
		},
	}
	for name, tc := range cases {
		if got := profile.Cause(tc.err); got != tc.want {
			t.Errorf("%s: %q, want %q", name, got, tc.want)
		}
	}
}
