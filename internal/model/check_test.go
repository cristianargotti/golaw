package model_test

import (
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func validSpec() model.Spec {
	return model.Spec{
		ID:    "text.dashes",
		Rule:  "rules/3@1",
		Scope: model.ScopeFile,
		Owner: model.OwnerCore,
		Needs: []string{"go"},
		Proof: "text.dashes",
	}
}

func TestScopeAndModeNames(t *testing.T) {
	t.Parallel()
	scopes := map[model.Scope]string{
		model.ScopeFile: "file", model.ScopeTree: "tree", model.ScopeEvent: "event",
		model.ScopeCommit: "commit", model.Scope(0): "unknown", model.Scope(9): "unknown",
	}
	for scope, want := range scopes {
		if scope.String() != want {
			t.Errorf("Scope(%d).String() = %q, want %q", int(scope), scope.String(), want)
		}
		if scope.Valid() != (want != "unknown") {
			t.Errorf("Scope(%d).Valid() = %v", int(scope), scope.Valid())
		}
	}
	modes := map[model.Mode]string{
		model.ModeScan: "scan", model.ModeStaged: "staged", model.ModeHook: "hook",
		model.ModeCommit: "commit", model.Mode(0): "unknown", model.Mode(7): "unknown",
	}
	for mode, want := range modes {
		if mode.String() != want {
			t.Errorf("Mode(%d).String() = %q, want %q", int(mode), mode.String(), want)
		}
		if mode.Valid() != (want != "unknown") {
			t.Errorf("Mode(%d).Valid() = %v", int(mode), mode.Valid())
		}
	}
}

func TestListsAreSorted(t *testing.T) {
	t.Parallel()
	for _, list := range [][]string{model.Capabilities(), model.Requirements()} {
		for i := 1; i < len(list); i++ {
			if list[i-1] >= list[i] {
				t.Errorf("list %v is not sorted and unique", list)
			}
		}
	}
}

func TestSpecValidate(t *testing.T) {
	t.Parallel()
	if err := validSpec().Validate(); err != nil {
		t.Fatalf("a valid spec is refused: %v", err)
	}
	cases := map[string]func(s *model.Spec){
		"empty id":            func(s *model.Spec) { s.ID = "" },
		"id without family":   func(s *model.Spec) { s.ID = "dashes" },
		"id in upper case":    func(s *model.Spec) { s.ID = "Text.Dashes" },
		"rule without ref":    func(s *model.Spec) { s.Rule = "3" },
		"rule without at":     func(s *model.Spec) { s.Rule = "rules/3" },
		"zero scope":          func(s *model.Spec) { s.Scope = 0 },
		"scope out of range":  func(s *model.Spec) { s.Scope = model.ScopeCommit + 1 },
		"empty owner":         func(s *model.Spec) { s.Owner = "" },
		"owner in upper case": func(s *model.Spec) { s.Owner = "Core" },
		"empty proof":         func(s *model.Spec) { s.Proof = "" },
		"unknown need":        func(s *model.Spec) { s.Needs = []string{"web"} },
		"repeated need":       func(s *model.Spec) { s.Needs = []string{"go", "go"} },
		"unknown requirement": func(s *model.Spec) { s.Requires = []string{"network"} },
		"bad option":          func(s *model.Spec) { s.Options = model.OptionSchema{{Name: ""}} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := validSpec()
			mutate(&s)
			err := s.Validate()
			if !errors.Is(err, model.ErrInvalidSpec) {
				t.Fatalf("Validate() = %v, want ErrInvalidSpec", err)
			}
		})
	}
}

func TestSpecValidateAcceptsEveryKnownName(t *testing.T) {
	t.Parallel()
	s := validSpec()
	s.Needs = model.Capabilities()
	s.Requires = model.Requirements()
	s.Rule = "rules/4.2@7"
	s.Owner = "some-profile-name"
	s.Options = model.OptionSchema{{Name: "limit", Kind: model.OptionInt, Tighten: model.TightenDown, Default: model.IntValue(3)}}
	if err := s.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestValidIDAndCode(t *testing.T) {
	t.Parallel()
	if !model.ValidID("text.dashes") || model.ValidID("dashes") || model.ValidID("Text.dashes") || model.ValidID("") {
		t.Error("ValidID is wrong")
	}
	if !model.ValidCode("em-dash") || model.ValidCode("EM_DASH") || model.ValidCode("-x") || model.ValidCode("") {
		t.Error("ValidCode is wrong")
	}
}
