package proof_test

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// TestLoadManifestsJoinsThePartsOfTheProof pins contract D5: the proof
// reads several manifests as one, an entry may name the profile its
// fixtures run under, and an id in two files is refused, so the
// completeness pass still compares the whole registry with the whole proof.
func TestLoadManifestsJoinsThePartsOfTheProof(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "a.yml", "checks:\n  - {id: text.a, code: a-code, red: testdata/a/red, near: testdata/a/near}\n")
	write(t, root, "b.yml", "checks:\n  - {id: policy.b, code: b-code, red: testdata/b/red, near: testdata/b/near, profile: fixture}\n")
	write(t, root, "empty.yml", "checks: []\n")
	m, err := proof.LoadManifests(filepath.Join(root, "a.yml"), filepath.Join(root, "b.yml"), filepath.Join(root, "empty.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Names(); !slices.Equal(got, []string{"policy.b", "text.a"}) {
		t.Fatalf("Names() = %v", got)
	}
	if e, ok := m.Lookup("policy.b"); !ok || e.Profile != "fixture" {
		t.Fatalf("Lookup(policy.b) = %+v, %v", e, ok)
	}
	if e, _ := m.Lookup("text.a"); e.Profile != "" {
		t.Fatalf("an entry without a profile = %+v", e)
	}
}

func TestLoadManifestsRefusesWhatOneFileWouldRefuse(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "a.yml", "checks:\n  - {id: text.a, code: a-code, red: r, near: n}\n")
	write(t, root, "again.yml", "checks:\n  - {id: text.a, code: a-code, red: r2, near: n2}\n")
	write(t, root, "profile.yml", "checks:\n  - {id: text.c, code: c-code, red: r, near: n, profile: Not A Name}\n")
	for name, paths := range map[string][]string{
		"an id in two files":  {"a.yml", "again.yml"},
		"a malformed profile": {"profile.yml"},
		"an absent file":      {"a.yml", "absent.yml"},
		"no file":             nil,
	} {
		full := make([]string, 0, len(paths))
		for _, p := range paths {
			full = append(full, filepath.Join(root, p))
		}
		_, err := proof.LoadManifests(full...)
		if !errors.Is(err, proof.ErrManifest) {
			t.Errorf("%s: LoadManifests() = %v, want ErrManifest", name, err)
		}
		if name == "an id in two files" && (err == nil || !strings.Contains(err.Error(), "text.a")) {
			t.Errorf("the refusal must name the id: %v", err)
		}
	}
}

// ownedDash is the dash check of a profile: the engine applies it only
// while the policy enrolls that profile.
type ownedDash struct{ dashCheck }

func (o ownedDash) Spec() model.Spec {
	spec := o.dashCheck.Spec()
	spec.Owner = "fixture"
	return spec
}

// TestAProfileEntryRunsItsFixturesUnderTheProfile pins the profile of an
// entry: the fixtures of a check that applies only under a profile run with
// that profile enrolled, and the same check without it never reaches them.
func TestAProfileEntryRunsItsFixturesUnderTheProfile(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t)
	reg := registry(t, ownedDash{dashCheck{id: "text.dashes"}})
	entry := proof.Entry{ID: "text.dashes", Code: "em-dash", Red: "testdata/text.dashes/red", Near: "testdata/text.dashes/near", Profile: "fixture"}
	if failures := proof.Verify(t.Context(), reg, root, proof.Manifest{Checks: []proof.Entry{entry}}); len(failures) != 0 {
		t.Fatalf("Verify() under the profile = \n%s", reasons(failures))
	}
	entry.Profile = ""
	failures := proof.Baseline(t.Context(), reg, root, proof.Manifest{Checks: []proof.Entry{entry}})
	if len(failures) != 2 || !strings.Contains(failures[0].Reason, "the check was not-applicable") {
		t.Fatalf("Baseline() without the profile = %v", failures)
	}
	if p := proof.PolicyFor(entry); p.Profile != "" || len(p.Capabilities) != len(model.Capabilities()) {
		t.Fatalf("PolicyFor(core entry) = %+v", p)
	}
	entry.Profile = "fixture"
	if p := proof.PolicyFor(entry); p.Profile != "fixture" || p.Digest != proof.Policy().Digest {
		t.Fatalf("PolicyFor(profile entry) = %+v", p)
	}
}
