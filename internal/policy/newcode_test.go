package policy_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestCompileCarriesTheNewCodeFloor(t *testing.T) {
	t.Parallel()
	p, err := policy.Compile(decode(t, minimalPolicy+"checks: {coverage: {new_code: 90}}\n"), now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := p.OptionsFor(policy.CoverageCheck).Int("new_code"); !ok || got != 90 {
		t.Errorf("coverage.floors new_code = %d, %v, want 90", got, ok)
	}
	core, err := policy.Compile(decode(t, minimalPolicy), now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := core.OptionsFor(policy.CoverageCheck).Int("new_code"); ok {
		t.Error("a file without the floor sets it: the coverage package applies model.DefaultCoverageNew")
	}
}

func TestValidateKeepsTheNewCodeFloorAtOrAboveTheSonarWayValue(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		value  string
		refuse bool
	}{
		"below the Sonar way 80":    {value: "79", refuse: true},
		"over a hundred":            {value: "101", refuse: true},
		"the Sonar way (near miss)": {value: "80"},
		"a hundred (near miss)":     {value: "100"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := decode(t, minimalPolicy+"checks: {coverage: {new_code: "+tc.value+"}}\n").Validate(nil)
			if tc.refuse != (err != nil) {
				t.Fatalf("Validate = %v, refuse %v", err, tc.refuse)
			}
			if tc.refuse && (!errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), "checks.coverage.new_code")) {
				t.Errorf("error = %v, want ErrPolicy naming checks.coverage.new_code", err)
			}
		})
	}
}

func TestTheNewCodeFloorOnlyRises(t *testing.T) {
	t.Parallel()
	ninety := minimalPolicy + "checks: {coverage: {new_code: 90}}\n"
	tests := map[string]struct {
		inForce, proposed string
		weakens           bool
	}{
		"ninety to eighty-five":            {inForce: ninety, proposed: minimalPolicy + "checks: {coverage: {new_code: 85}}\n", weakens: true},
		"ninety to unset":                  {inForce: ninety, proposed: minimalPolicy, weakens: true},
		"unset to eighty-five (near miss)": {inForce: minimalPolicy, proposed: minimalPolicy + "checks: {coverage: {new_code: 85}}\n"},
		"ninety kept (near miss)":          {inForce: ninety, proposed: ninety},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := policy.CheckTightening(decode(t, tc.inForce), decode(t, tc.proposed), nil)
			if tc.weakens != (err != nil) {
				t.Fatalf("CheckTightening = %v, weakens %v", err, tc.weakens)
			}
			if tc.weakens && !strings.Contains(err.Error(), "checks.coverage.new_code: 90 to") {
				t.Errorf("error = %v, want the new_code weakening named", err)
			}
		})
	}
}

func TestTheNewCodeFloorEntersTheDigestOnlyWhenSet(t *testing.T) {
	t.Parallel()
	if got := policy.Digest(decode(t, fullPolicy)); got != fullDigest {
		t.Errorf("full digest = %s, want %s: a coverage block without new_code changed its digest", got, fullDigest)
	}
	if policy.Digest(decode(t, minimalPolicy+"checks: {coverage: {new_code: 90}}\n")) == policy.Digest(decode(t, minimalPolicy+"checks: {coverage: {}}\n")) {
		t.Error("the new_code floor is left out of the digest")
	}
}

func TestExplainNamesTheNewCodeFloor(t *testing.T) {
	t.Parallel()
	root, d := enrolled(t, minimalPolicy+"checks: {coverage: {new_code: 95}}\n")
	l, err := policy.Load(t.Context(), input(root, d))
	if err != nil {
		t.Fatal(err)
	}
	want := policy.Line{Key: "checks.coverage.floors.new_code", Value: "95", Source: "HEAD:golaw.yml"}
	if got := lines(policy.Explain(l))[want.Key]; got != want {
		t.Errorf("%s = %+v, want %+v", want.Key, got, want)
	}
}
