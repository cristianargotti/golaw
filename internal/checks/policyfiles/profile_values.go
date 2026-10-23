package policyfiles

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
)

type valuesAssertions struct{}

// ValuesAssertions is policy.values-assertions: the typed YAML assertions
// of the profile, each a scalar of a kind, optionally of a value, at a key
// path of every file its glob names outside testdata, with the missing key
// and the missing file judged by the rule the profile states (section 5.4).
func ValuesAssertions() model.Check { return valuesAssertions{} }

func (valuesAssertions) Spec() model.Spec { return spec("values-assertions", ruleProfile) }

func (valuesAssertions) Run(_ context.Context, in model.Input) (model.Result, error) {
	if status, ok := section(in, model.SectionAssertions); !ok {
		return model.Result{Status: status}, nil
	}
	documents := make(map[string]*yaml.Node)
	var findings []model.Finding
	for _, a := range in.Policy.ProfileData.Assertions {
		files := model.Select(in.Snapshot, func(p string) bool {
			matched, _ := path.Match(a.Path, p) //nolint:errcheck // the loader validated every glob
			return matched && in.Snapshot.Class(p) != model.ClassFixture
		})
		if len(files) == 0 && a.Files == model.FilesRequired {
			message := fmt.Sprintf("add %s: the profile asserts %s in it", a.Path, keyText(a.Key))
			findings = append(findings, finding(a.Path, 0, "values-file-missing", message))
		}
		for _, file := range files {
			root, err := valuesDocument(in, file.Path, documents)
			if err != nil {
				return answer(file.Path, err), nil
			}
			findings = append(findings, assertValue(root, file.Path, a)...)
		}
	}
	return model.Result{Status: model.Measured(), Scope: fmt.Sprintf("%d files", len(documents)), Findings: findings}, nil
}

// valuesDocument decodes a file once for every assertion that names it.
func valuesDocument(in model.Input, file string, documents map[string]*yaml.Node) (*yaml.Node, error) {
	if root, ok := documents[file]; ok {
		return root, nil
	}
	data, _, err := read(in, file)
	if err != nil {
		return nil, err
	}
	root, err := decodeYAML(file, data)
	if err != nil {
		return nil, err
	}
	documents[file] = root
	return root, nil
}

// assertValue judges one assertion over one decoded file: the key path,
// then the kind of the scalar, then its value when the profile gives one.
func assertValue(root *yaml.Node, file string, a model.Assertion) []model.Finding {
	segments, want := keyText(a.Key), a.Kind
	if a.Equals != nil {
		want += " " + *a.Equals
	}
	node := child(root, a.Key...)
	switch {
	case node == nil && a.Missing == model.MissingIgnore:
		return nil
	case node == nil:
		message := fmt.Sprintf("set %s in %s: the profile requires %s", segments, file, want)
		return []model.Finding{finding(file, 0, "values-key-missing", message)}
	case scalarKind(node) != a.Kind || a.Equals != nil && !sameScalar(node, a.Kind, *a.Equals):
		message := fmt.Sprintf("%s of %s is %s %s: the profile requires %s", segments, file, scalarKind(node), node.Value, want)
		return []model.Finding{finding(file, node.Line, "values-assertion", message)}
	}
	return nil
}

// scalarKind names the kind of a node the way an assertion names it.
func scalarKind(node *yaml.Node) string {
	switch {
	case node.Kind == yaml.MappingNode:
		return "mapping"
	case node.Kind == yaml.SequenceNode:
		return "sequence"
	case node.ShortTag() == "!!str":
		return model.AssertString
	default:
		return strings.TrimPrefix(node.ShortTag(), "!!")
	}
}

// sameScalar compares a scalar of the asserted kind with the expected text
// as values of that kind, so 0x258 equals 600 and True equals true.
func sameScalar(node *yaml.Node, kind, equals string) bool {
	switch kind {
	case model.AssertBool:
		got, err := strconv.ParseBool(node.Value)
		return err == nil && strconv.FormatBool(got) == equals
	case model.AssertInt:
		var got int64
		return node.Decode(&got) == nil && strconv.FormatInt(got, 10) == equals
	default:
		return node.Value == equals
	}
}

// keyText spells a key path for a person, a segment that holds a dot or a
// space quoted so the path reads unambiguously.
func keyText(segments []string) string {
	out := make([]string, 0, len(segments))
	for _, s := range segments {
		if strings.ContainsAny(s, ". ") {
			s = strconv.Quote(s)
		}
		out = append(out, s)
	}
	return strings.Join(out, ".")
}
