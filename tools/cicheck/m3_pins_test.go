package cicheck_test

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
)

var errM3TagUnavailable = errors.New("tag unavailable")

func TestM3PinDocumentsIncludeEveryRendering(t *testing.T) {
	t.Parallel()
	documents := m3PinDocuments(t)
	if len(documents) != len(m3Capabilities()) {
		t.Fatalf("pin documents=%d, want %d", len(documents), len(m3Capabilities()))
	}
	for name, data := range documents {
		pairs, err := m3PinPairs(data)
		if err != nil || len(pairs) == 0 {
			t.Fatalf("%s: pairs=%v, error=%v", name, pairs, err)
		}
	}
}

func TestM3PinPairsAndResolution(t *testing.T) {
	t.Parallel()
	const (
		checkout = "08c6903cd8c0fde910a37f88322edcfb5dd907a8"
		node     = "820762786026740c76f36085b0efc47a31fe5020"
		endpoint = "repos/actions/checkout/commits/v5.0.0"
		source   = `jobs:
  ci:
    steps:
      - uses: actions/checkout@08c6903cd8c0fde910a37f88322edcfb5dd907a8 # v5.0.0
`
	)
	want := map[string]string{endpoint: checkout}
	for _, test := range []struct {
		name, data string
		fail       bool
	}{
		{"near", source, false},
		{"floating", strings.ReplaceAll(source, checkout, "v5"), true},
		{"missing version", strings.ReplaceAll(source, " # v5.0.0", ""), true},
		{"broken YAML", "jobs: [\n", true},
		{"no pins", "jobs: {}\n", true},
		{"same pair twice", source + "      - uses: actions/checkout@" + checkout + " # v5.0.0\n", false},
		{"conflicting pair", source + "      - uses: actions/checkout@" + node + " # v5.0.0\n", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := m3PinPairs([]byte(test.data))
			if (err != nil) != test.fail {
				t.Fatalf("pairs=%v, error=%v", got, err)
			}
			if !test.fail && !maps.Equal(got, want) {
				t.Fatalf("pairs=%v, want %v", got, want)
			}
		})
	}
	for _, test := range []struct {
		name, sha string
		err       error
		fail      bool
	}{
		{"matching tag", checkout, nil, false},
		{"different commit", node, nil, true},
		{"empty answer", "", nil, true},
		{"unavailable tag", "", errM3TagUnavailable, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			requireM3PinResolution(t, want, endpoint, test.sha, test.err, test.fail)
		})
	}
}

func requireM3PinResolution(t *testing.T, want map[string]string, endpoint, sha string, failure error, fail bool) {
	t.Helper()
	calls := 0
	err := m3CheckPinPairs(want, func(got string) (string, error) {
		calls++
		if got != endpoint {
			t.Fatalf("endpoint=%q", got)
		}
		return sha, failure
	})
	if calls != 1 || (err != nil) != fail {
		t.Fatalf("calls=%d, error=%v", calls, err)
	}
	if fail && !strings.Contains(err.Error(), endpoint) {
		t.Fatalf("error does not name the tag: %v", err)
	}
}

func TestM3PinSubdirectoriesAndBoundaries(t *testing.T) {
	t.Parallel()
	const sha = "08c6903cd8c0fde910a37f88322edcfb5dd907a8"
	for _, test := range []struct {
		name, reference, comment string
		fail                     bool
	}{
		{"subdirectory", "actions/checkout/nested/action@" + sha, "# v5.0.0", false},
		{"local action", "./action", "# v5.0.0", true},
		{"uppercase SHA", "actions/checkout@" + strings.ToUpper(sha), "# v5.0.0", true},
		{"short version", "actions/checkout@" + sha, "# v5", true},
		{"version suffix", "actions/checkout@" + sha, "# v5.0.0 extra", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			data := "jobs:\n  ci:\n    steps:\n      - uses: " + test.reference + " " + test.comment + "\n"
			pairs, err := m3PinPairs([]byte(data))
			if (err != nil) != test.fail {
				t.Fatalf("pairs=%v, error=%v", pairs, err)
			}
			if !test.fail && pairs["repos/actions/checkout/commits/v5.0.0"] != sha {
				t.Fatalf("action subdirectory changed repository: %v", pairs)
			}
		})
	}
}

func TestM3PinResolutionOrderAndDetails(t *testing.T) {
	t.Parallel()
	const (
		first = "repos/actions/checkout/commits/v5.0.0"
		last  = "repos/actions/setup-node/commits/v7.0.0"
		sha   = "08c6903cd8c0fde910a37f88322edcfb5dd907a8"
	)
	pairs := map[string]string{last: sha, first: sha}
	var calls []string
	err := m3CheckPinPairs(pairs, func(endpoint string) (string, error) {
		calls = append(calls, endpoint)
		return "\n " + sha + " \n", nil
	})
	if err != nil || !slices.Equal(calls, []string{first, last}) {
		t.Fatalf("calls=%v, error=%v", calls, err)
	}
	for _, failure := range []error{nil, errM3TagUnavailable} {
		err := m3CheckPinPairs(map[string]string{first: sha}, func(string) (string, error) {
			return "wrong-sha", failure
		})
		if err == nil || !strings.Contains(err.Error(), first) || !strings.Contains(err.Error(), sha) {
			t.Fatalf("error does not name endpoint and expected SHA: %v", err)
		}
		if failure == nil && !strings.Contains(err.Error(), "wrong-sha") {
			t.Fatalf("error does not name received SHA: %v", err)
		}
		if failure != nil && !errors.Is(err, failure) {
			t.Fatalf("resolver failure was not preserved: %v", err)
		}
	}
}
