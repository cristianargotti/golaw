package main

import (
	"path/filepath"
	"testing"
)

// TestM5PreparationInstallsTheToolsAndTheOverlay drives the preparation of
// the M5 candidate through the simulated answers of the M3 one: the tools
// installed once, the candidate's proxy ahead of the public one, the
// fixture database behind govulncheck, and no make looked up, because no
// M5 row runs one.
func TestM5PreparationInstallsTheToolsAndTheOverlay(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, version string
		fail          bool
	}{
		{"near the M5 candidate", m5CandidateVersion, false},
		{"red another version", m3CandidateVersion, true},
	} {
		s := m2TestSuite(t)
		s.m2.Ready = false
		s.execute = newM2Script(t, s, m3PrepareAnswer(t, s, tc.version)).Execute
		err := s.prepareM5(t.Context())
		if (err != nil) != tc.fail {
			t.Fatalf("%s: prepare = %v", tc.name, err)
		}
		if tc.fail {
			continue
		}
		proxy, err := m2FileURL(filepath.Join(s.base, m5ProxyDir))
		if err != nil {
			t.Fatal(err)
		}
		if !s.m2.Ready || environmentValue(s.env, "GOPROXY") != proxy+",https://proxy.golang.org,direct" ||
			environmentValue(s.env, m2DatabaseKey) == "" || environmentValue(s.env, "GOBIN") != s.m2.Bin ||
			s.m2.Make != "" || len(s.digest) != 64 {
			t.Fatalf("%s: environment %q, make %q, digest %q", tc.name, s.env, s.m2.Make, s.digest)
		}
		if got := environmentValue(s.m5Scope("m5-sample", "case").env, "GOPROXY"); got != environmentValue(s.env, "GOPROXY") {
			t.Fatalf("%s: a case lost the overlay: GOPROXY = %q", tc.name, got)
		}
	}
}
