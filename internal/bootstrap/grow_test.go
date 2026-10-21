package bootstrap_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
)

const reviewedHead = "golaw: 1\nkit: { version: 0.1.0 }\nlanguage: en\n"

func TestGrowCapabilitiesAddsTheNamedCapabilitiesToTheLine(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		current string
		names   []string
		want    string
	}{
		"one more":               {reviewedHead + "capabilities: [go]\nladder: {}\n", []string{"api"}, reviewedHead + "capabilities: [api, go]\nladder: {}\n"},
		"sorted and unique":      {reviewedHead + "capabilities: [go, cli]\n", []string{"db", "cli", "db"}, reviewedHead + "capabilities: [cli, db, go]\n"},
		"a comment kept":         {reviewedHead + "capabilities: [go] # reviewed\n", []string{"image"}, reviewedHead + "capabilities: [go, image] # reviewed\n"},
		"an empty line":          {reviewedHead + "capabilities: []\n", []string{"image"}, reviewedHead + "capabilities: [image]\n"},
		"no line at all":         {reviewedHead, []string{"go", "hexagonal"}, reviewedHead + "capabilities: [go, hexagonal]\n"},
		"no final newline":       {"golaw: 1\nkit: { version: 0.1.0 }", []string{"go"}, "golaw: 1\nkit: { version: 0.1.0 }\ncapabilities: [go]\n"},
		"nothing new":            {reviewedHead + "capabilities: [go, cli]\n", []string{"cli"}, reviewedHead + "capabilities: [go, cli]\n"},
		"no flag":                {reviewedHead + "capabilities: [go]\n", nil, reviewedHead + "capabilities: [go]\n"},
		"spaces inside":          {reviewedHead + "capabilities:   [ go ]\n", []string{"db"}, reviewedHead + "capabilities: [db, go]\n"},
		"a capability elsewhere": {reviewedHead + "# capabilities: [web]\ncapabilities: [go]\n", []string{"api"}, reviewedHead + "# capabilities: [web]\ncapabilities: [api, go]\n"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := bootstrap.GrowCapabilities([]byte(tc.current), tc.names)
			if err != nil || string(got) != tc.want {
				t.Fatalf("GrowCapabilities = %q, %v, want %q", got, err, tc.want)
			}
		})
	}
}

func TestGrowCapabilitiesRefusesWhatItCannotAdd(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		current string
		names   []string
		want    string
	}{
		"unknown":       {reviewedHead + "capabilities: [go]\n", []string{"web"}, `usage: unknown capability "web"`},
		"a shape alone": {reviewedHead + "capabilities: [image]\n", []string{"db"}, "usage: invalid policy: capability db needs go: add go to the capabilities"},
		"a block list":  {reviewedHead + "capabilities:\n  - go\n", []string{"api"}, "usage: golaw.yml lists its capabilities in a form init does not edit; write capabilities: [api, go] in golaw.yml by hand"},
		"not a policy":  {"capabilities: [go\n", []string{"api"}, "decode golaw.yml"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := bootstrap.GrowCapabilities([]byte(tc.current), tc.names)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("GrowCapabilities = %v, want %q", err, tc.want)
			}
			if name != "not a policy" && !errors.Is(err, model.ErrUsage) {
				t.Fatalf("%v is no usage error", err)
			}
		})
	}
}

func TestGrowCapabilitiesRefusesUnsupportedEmptyKeysAndDocumentEnd(t *testing.T) {
	t.Parallel()
	for name, suffix := range map[string]string{
		"quoted empty key": "\"capabilities\": []\n",
		"spaced empty key": "capabilities : []\n",
		"quoted null key":  "'capabilities': null\n",
		"document end":     "...\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := bootstrap.GrowCapabilities([]byte(reviewedHead+suffix), []string{"go"})
			want := "usage: golaw.yml lists its capabilities in a form init does not edit; write capabilities: [go] in golaw.yml by hand"
			if !errors.Is(err, model.ErrUsage) || err.Error() != want || got != nil {
				t.Fatalf("GrowCapabilities = %q, %v, want %q", got, err, want)
			}
		})
	}
}
