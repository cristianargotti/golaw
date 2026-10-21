package policy_test

import (
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// floorRefusal is the text Validate answers for a Go shape without go.
func floorRefusal(name string) string {
	return "invalid policy: capability " + name + " needs go: add go to the capabilities"
}

func TestValidateRefusesAGoShapeWithoutGo(t *testing.T) {
	t.Parallel()
	refused := map[string]struct {
		line string
		want string
	}{
		"api":                 {"capabilities: [api]\n", "api"},
		"cli":                 {"capabilities: [cli]\n", "cli"},
		"db":                  {"capabilities: [db]\n", "db"},
		"hexagonal":           {"capabilities: [hexagonal]\n", "hexagonal"},
		"the first in order":  {"capabilities: [hexagonal, db, api, image]\n", "api"},
		"beside an image":     {"capabilities: [image, cli]\n", "cli"},
		"db before hexagonal": {"capabilities: [hexagonal, db]\n", "db"},
	}
	for name, tc := range refused {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := decode(t, minimalPolicy+tc.line).Validate(schemas())
			if !errors.Is(err, model.ErrPolicy) || err.Error() != floorRefusal(tc.want) {
				t.Fatalf("Validate = %v, want %q", err, floorRefusal(tc.want))
			}
		})
	}
}

func TestValidateAcceptsEveryCapabilitySetThatHoldsTheFloor(t *testing.T) {
	t.Parallel()
	for name, line := range map[string]string{
		"no line":           "",
		"empty":             "capabilities: []\n",
		"go alone":          "capabilities: [go]\n",
		"image alone":       "capabilities: [image]\n",
		"a shape with go":   "capabilities: [api, go]\n",
		"go first":          "capabilities: [go, hexagonal]\n",
		"every capability":  "capabilities: [api, cli, db, go, hexagonal, image]\n",
		"image beside go":   "capabilities: [image, go]\n",
		"the kit's own set": "capabilities: [cli, go]\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := decode(t, minimalPolicy+line).Validate(schemas()); err != nil {
				t.Fatalf("Validate = %v", err)
			}
		})
	}
}

func TestTheFloorFollowsTheKnownNameCheck(t *testing.T) {
	t.Parallel()
	err := decode(t, minimalPolicy+"capabilities: [web, api]\n").Validate(nil)
	if !errors.Is(err, model.ErrPolicy) || err.Error() != `invalid policy: capabilities names unknown "web"` {
		t.Fatalf("Validate = %v", err)
	}
	if _, err := policy.Compile(decode(t, minimalPolicy+"capabilities: [db]\n"), now(), nil); err == nil || err.Error() != floorRefusal("db") {
		t.Fatalf("Compile = %v, want the floor refused before any value compiles", err)
	}
}

func TestCheckCapabilitiesAnswersTheFloorOfASet(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		set  []string
		want string
	}{
		"nil":              {nil, ""},
		"go":               {[]string{"go"}, ""},
		"image":            {[]string{"image"}, ""},
		"unsorted with go": {[]string{"hexagonal", "go", "api"}, ""},
		"cli":              {[]string{"cli"}, floorRefusal("cli")},
		"api and db":       {[]string{"db", "api"}, floorRefusal("api")},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := policy.CheckCapabilities(tc.set)
			got := ""
			if err != nil {
				got = err.Error()
			}
			if got != tc.want || (err != nil && !errors.Is(err, model.ErrPolicy)) {
				t.Fatalf("CheckCapabilities(%v) = %v, want %q", tc.set, err, tc.want)
			}
		})
	}
}
