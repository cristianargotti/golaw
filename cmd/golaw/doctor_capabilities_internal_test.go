package main

import (
	"strings"
	"testing"
)

func TestDoctorNamesTheCapabilitiesInForce(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		line string
		want string
	}{
		"the rendered go": {"capabilities: [go]\n", "doctor.capabilities capabilities: ok [capabilities-in-force] go\n"},
		"three":           {"capabilities: [api, go, image]\n", "doctor.capabilities capabilities: ok [capabilities-in-force] api, go, image\n"},
		"none at all":     {"capabilities: []\n", "doctor.capabilities capabilities: ok [capabilities-in-force] none\n"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := capabilityRepository(t, tc.line)
			ta, _ := doctorApp(t)
			if code := ta.run(t.Context(), []string{"doctor", root}); code != exitClean {
				t.Fatalf("exit = %d, stderr = %q", code, ta.stderr.String())
			}
			if got := strings.Count(ta.stdout.String(), tc.want); got != 1 {
				t.Fatalf("capability lines = %d in %q", got, ta.stdout.String())
			}
		})
	}
}

func TestDoctorNamesNoCapabilityWithoutAPolicyInForce(t *testing.T) {
	t.Parallel()
	refused := capabilityRepository(t, "capabilities: [api]\n")
	outside := t.TempDir()
	for name, root := range map[string]string{"a policy doctor cannot load": refused, "outside git": outside} {
		ta, _ := doctorApp(t)
		if code := ta.run(t.Context(), []string{"doctor", root}); code != exitClean {
			t.Fatalf("%s: exit = %d, stderr = %q", name, code, ta.stderr.String())
		}
		if strings.Contains(ta.stdout.String(), "doctor.capabilities") {
			t.Errorf("%s: %q", name, ta.stdout.String())
		}
	}
}
