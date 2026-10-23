package policyfiles

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

// TestSectionGatesAProfileCheckOnItsData pins contract D5: a check the
// profile data turns on answers profiled's status without a profile,
// NotApplicable naming the section when the enrolled profile omits it, and
// applies only when the data carries the section.
func TestSectionGatesAProfileCheckOnItsData(t *testing.T) {
	t.Parallel()
	core := model.CoreDefaults()
	if status, ok := section(model.Input{Policy: &core}, model.SectionTerms); ok || status.Kind != model.StatusNotApplicable {
		t.Errorf("without a profile: %+v, %v", status, ok)
	}
	enrolled := model.CoreDefaults()
	enrolled.Profile = "fixture"
	status, ok := section(model.Input{Policy: &enrolled}, model.SectionTargets)
	if ok || status.Kind != model.StatusNotApplicable || status.Reason != "the profile carries no targets" {
		t.Errorf("with a profile and no data: %+v, %v", status, ok)
	}
	enrolled.ProfileData = &model.ProfileData{Name: "fixture", Targets: []string{"image-lint"}}
	if status, ok := section(model.Input{Policy: &enrolled}, model.SectionTargets); !ok || status != (model.Status{}) {
		t.Errorf("with the section: %+v, %v", status, ok)
	}
	if status, ok := section(model.Input{Policy: &enrolled}, model.SectionPaths); ok || status.Reason != "the profile carries no paths" {
		t.Errorf("with another section only: %+v, %v", status, ok)
	}
}

// TestProfileChecksCarryTheProfileRule pins the rule reference and the
// family of every check the profile data turns on.
func TestProfileChecksCarryTheProfileRule(t *testing.T) {
	t.Parallel()
	if ruleProfile != "rules/4.14@1" {
		t.Errorf("ruleProfile = %s", ruleProfile)
	}
	for _, c := range profileChecks() {
		s := c.Spec()
		if !strings.HasPrefix(s.ID, family) || s.Rule != ruleProfile || s.Owner != model.OwnerCore {
			t.Errorf("%s: rule %s, owner %s", s.ID, s.Rule, s.Owner)
		}
	}
}
