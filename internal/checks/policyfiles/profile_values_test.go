package policyfiles_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// values is the fixture values file, an annotation and a security context
// the fixture profile asserts; jobs is a job file its optional glob covers.
const (
	values = "deploy/values.yaml"
	jobs   = "deploy/jobs/cleanup.yaml"
)

// TestValuesAssertions pins policy.values-assertions (contract D5): a
// typed scalar at a key path of every file a glob names, with the missing
// key and the missing file each judged by the explicit rule the profile
// gives, merge keys expanded as a tool expands them.
func TestValuesAssertions(t *testing.T) {
	t.Parallel()
	const id = "policy.values-assertions"
	data := profiletest.Data(t)
	red := depsProof(t, id, "red")
	red.data = data
	near := depsProof(t, id, "near")
	near.data = data
	annotation := "podAnnotations:\n  example.test/do-not-disrupt: \"true\"\n"
	assertDeps(t, policyfiles.ValuesAssertions(), []depsCase{
		{name: "value differs red", input: red, want: []string{"values-assertion"}},
		{name: "values equal, no job file near", input: near},
		{name: "string where a bool is asserted red", input: depsEdit(near, values, "securityContext:\n  runAsNonRoot: \"true\"\n"+annotation), want: []string{"values-assertion"}},
		{name: "bool where a string is asserted red", input: depsEdit(near, values, "securityContext:\n  runAsNonRoot: true\npodAnnotations:\n  example.test/do-not-disrupt: true\n"), want: []string{"values-assertion"}},
		{name: "mapping where a bool is asserted red", input: depsEdit(near, values, "securityContext:\n  runAsNonRoot:\n    enabled: true\n"+annotation), want: []string{"values-assertion"}},
		{name: "sequence where a bool is asserted red", input: depsEdit(near, values, "securityContext:\n  runAsNonRoot: [true]\n"+annotation), want: []string{"values-assertion"}},
		{name: "key absent red", input: depsEdit(near, values, "securityContext:\n  runAsNonRoot: true\n"), want: []string{"values-key-missing"}},
		{name: "parent not a mapping red", input: depsEdit(near, values, "securityContext: strict\n"+annotation), want: []string{"values-key-missing"}},
		{name: "required file absent red", input: depsEdit(near, values, ""), want: []string{"values-file-missing", "values-file-missing"}},
		{name: "job file with the key near", input: depsEdit(near, jobs, "spec:\n  activeDeadlineSeconds: 0x258\n")},
		{name: "job file without the key red", input: depsEdit(near, jobs, "spec:\n  schedule: daily\n"), want: []string{"values-key-missing"}},
		{name: "job key of another kind red", input: depsEdit(near, jobs, "spec:\n  activeDeadlineSeconds: \"600\"\n"), want: []string{"values-assertion"}},
		{name: "merge key near", input: depsEdit(near, values, "base: &base\n  runAsNonRoot: true\nsecurityContext:\n  <<: *base\n"+annotation)},
		{
			name:   "second document",
			input:  depsEdit(near, values, "securityContext:\n  runAsNonRoot: true\n---\nother: 1\n"),
			status: model.StatusUnmeasured, reason: values + ":3:", outside: true,
		},
		{name: "syntax error", input: depsEdit(near, values, "securityContext: [\n"), status: model.StatusUnmeasured, reason: "parse " + values},
		{
			name:   "YAML 1.1 boolean",
			input:  depsEdit(near, values, "securityContext:\n  runAsNonRoot: yes\n"+annotation),
			status: model.StatusUnmeasured, reason: values + ":2:", outside: true,
		},
		{
			name:   "YAML 1.1 boolean in another case",
			input:  depsEdit(near, values, "securityContext:\n  runAsNonRoot: OFF\n"+annotation),
			status: model.StatusUnmeasured, reason: values + ":2:", outside: true,
		},
		{
			name:   "template expression as a key",
			input:  depsEdit(near, values, "securityContext: {{ .Values.sc }}\n"+annotation),
			status: model.StatusUnmeasured, reason: values + ":1:", outside: true,
		},
		{name: "quoted yes where a bool is asserted red", input: depsEdit(near, values, "securityContext:\n  runAsNonRoot: \"yes\"\n"+annotation), want: []string{"values-assertion"}},
		{name: "YAML 1.1 words off the key path near", input: depsEdit(near, values, "securityContext:\n  runAsNonRoot: true\n  privileged: no\nextra:\n  {{ .Values.x }}: 1\n"+annotation)},
		{
			name:   "over the read bound",
			input:  depsEdit(near, values, strings.Repeat("#", model.MaxReadBytes+1)),
			status: model.StatusUnmeasured, reason: model.ErrTooLarge.Error(),
		},
	})
}

// TestValuesAssertionsHonorsTheExplicitRules pins the rules a profile
// chooses: a key absent under ignore, a value judged once present, an
// optional glob that matches only a fixture, and the gates of D5.
func TestValuesAssertionsHonorsTheExplicitRules(t *testing.T) {
	t.Parallel()
	near := depsProof(t, "policy.values-assertions", "near")
	two := "2"
	ignore := *profiletest.Data(t)
	ignore.Assertions = []model.Assertion{
		{Path: values, Key: []string{"replicaCount"}, Kind: model.AssertInt, Equals: &two, Missing: model.MissingIgnore, Files: model.FilesOptional},
		{Path: "*/values.yml", Key: []string{"image"}, Kind: model.AssertString, Missing: model.MissingFinding, Files: model.FilesOptional},
	}
	withData := func(f fixture, d *model.ProfileData) fixture { f.data = d; return f }
	noSection := *profiletest.Data(t)
	noSection.Assertions = nil
	assertDeps(t, policyfiles.ValuesAssertions(), []depsCase{
		{name: "key absent under ignore near", input: withData(near, &ignore)},
		{name: "key present under ignore red", input: withData(depsEdit(near, values, "replicaCount: 3\n"), &ignore), want: []string{"values-assertion"}},
		{name: "key equal under ignore near", input: withData(depsEdit(near, values, "replicaCount: 2\n"), &ignore)},
		{name: "glob matching a fixture only near", input: withData(depsEdit(near, "testdata/values.yml", "other: 1\n"), &ignore)},
		{name: "core only", input: near, status: model.StatusNotApplicable, reason: "no profile is enrolled"},
		{name: "no assertions section", input: withData(near, &noSection), status: model.StatusNotApplicable, reason: "the profile carries no assertions"},
	})
}
