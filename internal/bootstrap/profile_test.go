package bootstrap_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// pinnedRef is the reference of an installed bundle a test pins.
func pinnedRef() policy.ProfileRef {
	return policy.ProfileRef{
		Name: "fixture", Source: "https://example.test/profiles/fixture",
		Version: "1.2.3", Digest: "sha256:" + strings.Repeat("a", 64),
	}
}

// pinnedLine is the one flow line init writes for pinnedRef.
const pinnedLine = "profile: { name: fixture, source: https://example.test/profiles/fixture, version: 1.2.3, digest: sha256:" +
	"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa }"

func TestRenderWritesTheProfileBlockAfterTheKitPin(t *testing.T) {
	t.Parallel()
	ref := pinnedRef()
	in, err := (bootstrap.Inputs{KitVersion: "0.1.0", Language: "pt-BR", Capabilities: []string{"api"}, Layout: goLayout(), Profile: &ref}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	got, err := bootstrap.Render(in)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(renderedCore, "kit: { version: 0.1.0 }\n", "kit: { version: 0.1.0 }\n"+pinnedLine+"\n", 1)
	if string(got) != want {
		t.Fatalf("rendered:\n%s\nwant:\n%s", got, want)
	}
	f, err := policy.Decode(got)
	if err != nil || f.Profile == nil || *f.Profile != ref {
		t.Fatalf("decoded profile = %+v, %v", f.Profile, err)
	}
	core, err := bootstrap.Render(bootstrap.Inputs{KitVersion: "0.1.0", Language: "pt-BR", Capabilities: []string{"api", "go"}, Layout: goLayout()})
	if err != nil || string(core) != renderedCore {
		t.Fatalf("core rendered:\n%s\n%v", core, err)
	}
}

func TestNormalizeRefusesAnIncompletePin(t *testing.T) {
	t.Parallel()
	for name, mutate := range map[string]func(*policy.ProfileRef){
		"no name":    func(r *policy.ProfileRef) { r.Name = "" },
		"no source":  func(r *policy.ProfileRef) { r.Source = "" },
		"no version": func(r *policy.ProfileRef) { r.Version = "" },
		"no digest":  func(r *policy.ProfileRef) { r.Digest = "" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ref := pinnedRef()
			mutate(&ref)
			_, err := (bootstrap.Inputs{KitVersion: "0.1.0", Language: "en", Layout: goLayout(), Profile: &ref}).Normalize()
			if !errors.Is(err, model.ErrUsage) || !strings.Contains(err.Error(), "profile pin") {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestSetProfileWritesTheOneFlowLine(t *testing.T) {
	t.Parallel()
	old := "profile: { name: fixture, source: https://example.test/profiles/fixture, version: 1.0.0, digest: sha256:" + strings.Repeat("b", 64) + " }\n"
	cases := map[string]struct {
		current string
		want    string
	}{
		"a fresh policy":      {renderedCore, strings.Replace(renderedCore, "kit: { version: 0.1.0 }\n", "kit: { version: 0.1.0 }\n"+pinnedLine+"\n", 1)},
		"a flow line moved":   {reviewedHead + old + "capabilities: [go]\n", reviewedHead + pinnedLine + "\ncapabilities: [go]\n"},
		"the same pin":        {reviewedHead + pinnedLine + "\n", reviewedHead + pinnedLine + "\n"},
		"a comment kept":      {"golaw: 1\nkit: { version: 0.1.0 } # pinned\nlanguage: en\n", "golaw: 1\nkit: { version: 0.1.0 } # pinned\n" + pinnedLine + "\nlanguage: en\n"},
		"no kit line":         {"golaw: 1\ncapabilities: [go]\n", "golaw: 1\ncapabilities: [go]\n" + pinnedLine + "\n"},
		"no final newline":    {"golaw: 1\nkit: { version: 0.1.0 }", "golaw: 1\nkit: { version: 0.1.0 }\n" + pinnedLine + "\n"},
		"a profile elsewhere": {"# profile: { name: old }\n" + reviewedHead + old, "# profile: { name: old }\n" + reviewedHead + pinnedLine + "\n"},
		"a document end":      {reviewedHead + "...\n", "golaw: 1\nkit: { version: 0.1.0 }\n" + pinnedLine + "\nlanguage: en\n...\n"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := bootstrap.SetProfile([]byte(tc.current), pinnedRef())
			if err != nil || string(got) != tc.want {
				t.Fatalf("SetProfile = %q, %v, want %q", got, err, tc.want)
			}
		})
	}
}

func TestSetProfileRefusesWhatItCannotEdit(t *testing.T) {
	t.Parallel()
	byHand := "usage: golaw.yml names its profile in a form init does not edit; write " + pinnedLine + " in golaw.yml by hand"
	cases := map[string]struct {
		current string
		want    string
		usage   bool
	}{
		"a block form": {reviewedHead + "profile:\n  name: fixture\n  source: s\n  version: 1.0.0\n  digest: sha256:" + strings.Repeat("c", 64) + "\n", byHand, true},
		"a quoted key": {reviewedHead + "\"profile\": null\n", byHand, true},
		"a second doc": {reviewedHead + "...\n---\nprofile: { name: old }\n", "decode golaw.yml", false},
		"not a policy": {"capabilities: [go\n", "decode golaw.yml", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := bootstrap.SetProfile([]byte(tc.current), pinnedRef())
			if err == nil || !strings.Contains(err.Error(), tc.want) || got != nil {
				t.Fatalf("SetProfile = %q, %v, want %q", got, err, tc.want)
			}
			if errors.Is(err, model.ErrUsage) != tc.usage {
				t.Fatalf("usage error = %v for %v", !tc.usage, err)
			}
		})
	}
}
