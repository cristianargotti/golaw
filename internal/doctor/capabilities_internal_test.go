package doctor

import "testing"

func TestAssessCapabilitiesNamesTheCapabilitiesInForce(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		capabilities []string
		detail       string
	}{
		"none":  {nil, "none"},
		"empty": {[]string{}, "none"},
		"go":    {[]string{"go"}, "go"},
		"three": {[]string{"api", "go", "image"}, "api, go, image"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			want := Line{ID: "doctor.capabilities", Key: "capabilities", State: "ok", Code: "capabilities-in-force", Detail: tc.detail}
			if got := AssessCapabilities(tc.capabilities); got != want {
				t.Fatalf("AssessCapabilities(%v) = %+v, want %+v", tc.capabilities, got, want)
			}
		})
	}
}
