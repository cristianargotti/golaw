package model_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestCoreDefaults(t *testing.T) {
	t.Parallel()
	p := model.CoreDefaults()
	if p.Sizes.FileLines != 300 || p.Sizes.FunctionLines != 40 || p.Language != "pt-BR" {
		t.Errorf("CoreDefaults() = %+v", p)
	}
	if p.Profile != "" || len(p.Capabilities) != 0 || p.Enrollment.State != model.EnrollmentNone {
		t.Errorf("CoreDefaults() must be core only and unenrolled: %+v", p)
	}
	if p.Has("go") {
		t.Error("no capability is enabled by default")
	}
	if got := p.OptionsFor("text.dashes"); len(got) != 0 {
		t.Errorf("OptionsFor on a nil map = %v", got)
	}
}

func TestPolicyLookups(t *testing.T) {
	t.Parallel()
	p := model.CoreDefaults()
	p.Capabilities = []string{"cli", "go"}
	p.Options = map[string]model.Options{"goast.file-length": {"limit": model.IntValue(300)}}
	p.Exceptions = []model.Exception{{Check: "goast.secret-fields", Path: "internal/auth/session.go", Record: "docs/adr/0009-session-token.md"}}
	if !p.Has("go") || p.Has("api") {
		t.Error("Has is wrong")
	}
	if v, ok := p.OptionsFor("goast.file-length").Int("limit"); !ok || v != 300 {
		t.Errorf("OptionsFor = %d, %v", v, ok)
	}
	if got := p.OptionsFor("text.dashes"); got != nil {
		t.Errorf("OptionsFor an unknown id = %v", got)
	}
	record, ok := p.Excepted("goast.secret-fields", "internal/auth/session.go")
	if !ok || record != "docs/adr/0009-session-token.md" {
		t.Errorf("Excepted = %q, %v", record, ok)
	}
	if _, ok := p.Excepted("goast.secret-fields", "internal/auth/other.go"); ok {
		t.Error("another path must not be excepted")
	}
	if _, ok := p.Excepted("goast.wall-clock", "internal/auth/session.go"); ok {
		t.Error("another check on the excepted path must not be excepted")
	}
}

func TestEnrollmentStateNames(t *testing.T) {
	t.Parallel()
	names := map[model.EnrollmentState]string{
		model.EnrollmentNone: "unenrolled", model.EnrollmentEnrolled: "enrolled",
		model.EnrollmentMismatch: "mismatch", model.EnrollmentMissing: "missing-policy",
		model.EnrollmentState(0): "unknown",
	}
	for state, want := range names {
		if state.String() != want {
			t.Errorf("EnrollmentState(%d).String() = %q, want %q", int(state), state.String(), want)
		}
	}
}
