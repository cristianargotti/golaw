package policyfiles

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestSpecIsATreeCheckOfTheFamily(t *testing.T) {
	t.Parallel()
	got := spec("makefile", ruleLadder, "go")
	want := model.Spec{ID: "policy.makefile", Rule: "rules/4.13@1", Scope: model.ScopeTree, Owner: model.OwnerCore, Needs: []string{"go"}, Proof: "policy.makefile"}
	if got.ID != want.ID || got.Rule != want.Rule || got.Scope != want.Scope || got.Owner != want.Owner ||
		!slices.Equal(got.Needs, want.Needs) || got.Proof != want.Proof {
		t.Fatalf("spec = %+v, want %+v", got, want)
	}
	for _, rule := range []string{ruleLinters, ruleTests, ruleSecurity, ruleSupply, ruleData, ruleContract, ruleDelivery, ruleLadder} {
		if err := spec("sample", rule).Validate(); err != nil {
			t.Errorf("rule %s: %v", rule, err)
		}
	}
}

func TestChecksJoinsTheThreeHalvesInOrder(t *testing.T) {
	t.Parallel()
	ids := func(checks []model.Check) []string {
		out := make([]string, 0, len(checks))
		for _, c := range checks {
			out = append(out, c.Spec().ID)
		}
		return out
	}
	want := ids(slices.Concat(lintChecks(), buildChecks(), depsChecks(), profileChecks()))
	if got := ids(Checks()); !slices.Equal(got, want) {
		t.Fatalf("Checks() = %v, want %v", got, want)
	}
}

func TestReadAnswersTheSnapshotAndItsAbsence(t *testing.T) {
	t.Parallel()
	in := model.Input{Snapshot: proof.NewTree(map[string][]byte{"Makefile": []byte("ci:\n")})}
	data, found, err := read(in, "Makefile")
	if err != nil || !found || string(data) != "ci:\n" {
		t.Fatalf("read(Makefile) = %q, %v, %v", data, found, err)
	}
	if data, found, err := read(in, "golaw.mk"); err != nil || found || data != nil {
		t.Fatalf("read(golaw.mk) = %q, %v, %v", data, found, err)
	}
	big := model.Input{Snapshot: proof.NewTree(map[string][]byte{"big.mk": make([]byte, model.MaxReadBytes+1)})}
	if _, found, err := read(big, "big.mk"); found || !errors.Is(err, model.ErrTooLarge) || !strings.Contains(err.Error(), "big.mk") {
		t.Fatalf("read(big.mk) = %v, %v", found, err)
	}
}

func TestAnswerTurnsEveryErrorIntoTheResultOfTheCheck(t *testing.T) {
	t.Parallel()
	bound := answer("big.mk", fmt.Errorf("read big.mk: %w", model.ErrTooLarge))
	if bound.Status.Kind != model.StatusMeasured || !slices.Equal(bound.Unread, []string{"big.mk"}) {
		t.Errorf("over the read bound = %+v", bound)
	}
	limit := answer("Makefile", fmt.Errorf("read the Makefile: %w", outside("Makefile", 12, "$(shell) in the prerequisites of ci")))
	want := "read the Makefile: Makefile:12: $(shell) in the prerequisites of ci: outside the declared subset, a stated limit; docs/rules.md names the rewrite"
	if !model.IsOutsideSubset(limit.Status) || limit.Status.Reason != want {
		t.Errorf("a stated limit = %+v", limit)
	}
	other := answer("Makefile", fmt.Errorf("read Makefile: %w", model.ErrUnreadable))
	if other.Status.Kind != model.StatusUnmeasured || model.IsOutsideSubset(other.Status) || other.Status.Reason != "read Makefile: path unreadable" {
		t.Errorf("an unreadable file = %+v", other)
	}
	if !errors.Is(outside("a.yml", 1, "x"), model.ErrOutsideSubset) {
		t.Error("outside must wrap ErrOutsideSubset")
	}
}

func TestProfiledWaitsForAProfile(t *testing.T) {
	t.Parallel()
	core := model.CoreDefaults()
	status, ok := profiled(model.Input{Policy: &core})
	if ok || status.Kind != model.StatusNotApplicable || status.Reason != "no profile is enrolled; the profile turns this rule on" {
		t.Errorf("core only = %+v, %v", status, ok)
	}
	enrolled := model.CoreDefaults()
	enrolled.Profile = "fixture"
	if _, ok := profiled(model.Input{Policy: &enrolled}); !ok {
		t.Error("an enrolled profile must apply")
	}
}

func TestFindingIsADenialAtItsLine(t *testing.T) {
	t.Parallel()
	got := finding("Makefile", 3, "ci-target-missing", "the ci target lacks vet")
	want := model.Finding{Path: "Makefile", Line: 3, Code: "ci-target-missing", Message: "the ci target lacks vet", Severity: model.SeverityDeny}
	if got != want {
		t.Fatalf("finding = %+v", got)
	}
}

const mergedConfig = `base: &base
  enable: [a, b]
  default: none
linters:
  <<: *base
  default: standard
  disable:
    - c
    - d
alias: *base
`

func TestDecodeYAMLExpandsMergeKeysAndFollowsAliases(t *testing.T) {
	t.Parallel()
	root, err := decodeYAML(".golangci.yml", []byte(mergedConfig))
	if err != nil {
		t.Fatal(err)
	}
	if got := child(root, "linters", "default"); got == nil || got.Value != "standard" {
		t.Errorf("the own key must win over the merged one: %+v", got)
	}
	if got := scalars(child(root, "linters", "enable")); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("merged enable = %v", got)
	}
	if got := scalars(child(root, "linters", "disable")); !slices.Equal(got, []string{"c", "d"}) {
		t.Errorf("block disable = %v", got)
	}
	if got := scalars(child(root, "alias", "enable")); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("aliased enable = %v", got)
	}
	if child(root, "linters", "absent") != nil || child(root, "linters", "default", "deeper") != nil || scalars(child(root, "linters")) != nil {
		t.Error("an absent key, a scalar walked into or a mapping read as a list must answer nil")
	}
	keys := make([]string, 0, 3)
	for _, e := range entries(child(root, "linters")) {
		keys = append(keys, fmt.Sprintf("%s:%d", e.key, e.line))
	}
	if !slices.Equal(keys, []string{"default:6", "disable:7", "enable:2"}) {
		t.Errorf("entries = %v", keys)
	}
	if entries(child(root, "linters", "enable")) != nil {
		t.Error("the entries of a sequence must be nil")
	}
}

// TestDecodeYAMLMergesASequenceOfAnchorsInOrder pins the precedence of a
// merge of several anchors: the own key first, then the first anchor that
// holds the key.
func TestDecodeYAMLMergesASequenceOfAnchorsInOrder(t *testing.T) {
	t.Parallel()
	text := "a: &a {x: 1, y: 1}\nb: &b {y: 2, z: 2}\nc:\n  <<: [*a, *b, plain]\n  x: 3\n"
	root, err := decodeYAML("c.yml", []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, 3)
	for _, e := range entries(child(root, "c")) {
		got = append(got, e.key+"="+e.value.Value)
	}
	if !slices.Equal(got, []string{"x=3", "y=1", "z=2"}) {
		t.Fatalf("merged = %v", got)
	}
}

func TestDecodeYAMLRefusesWhatItCannotRead(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"", "# nothing\n"} {
		if root, err := decodeYAML("empty.yml", []byte(text)); err != nil || root.Kind != yaml.MappingNode || len(root.Content) != 0 {
			t.Errorf("an empty document %q = %v, %v", text, root, err)
		}
	}
	for name, text := range map[string]string{
		"a sequence root": "- a\n- b\n",
		"two documents":   "a: 1\n---\nb: 2\n",
	} {
		if _, err := decodeYAML("a.yml", []byte(text)); !errors.Is(err, model.ErrOutsideSubset) || !strings.HasPrefix(err.Error(), "a.yml:") {
			t.Errorf("%s = %v, want a stated limit naming the file", name, err)
		}
	}
	_, err := decodeYAML("a.yml", []byte("a: [\n"))
	if err == nil || errors.Is(err, model.ErrOutsideSubset) || !strings.Contains(err.Error(), "a.yml") {
		t.Errorf("a syntax error = %v, want a parse error naming the file", err)
	}
}
