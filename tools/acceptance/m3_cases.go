package main

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
	scanreport "github.com/cristianargotti/golaw/internal/report"
)

const (
	m3CasesCategory = "m3-cases"
	m3LintFile      = ".golangci.yml"
	m3LintersKey    = "linters"
	m3GitleaksFile  = ".gitleaks.toml"
	m3GolangciCheck = "policy.golangci"
	m3MakefileCheck = "policy.makefile"
	m3MovedTarget   = "deadcode"
	m3QualityFile   = "mk/quality.mk"
	m3SecondModule  = "example.test/second"
	m3SecondRecord  = "docs/adr/0002-second-module.md"
	// m3SecondRecordText is the smallest record policy.dependency-records
	// accepts: the check reads the front matter alone, so the body needs
	// nothing beyond its heading.
	m3SecondRecordText = "---\nrecord: 2\ntitle: second module\nstatus: accepted\nmodules:\n  - " +
		m3SecondModule + "\n---\n\n# Second module\n"
)

// m3Case is one of the six cases section 11 names: the check it proves, the
// edit that check must find and the near edit of the same shape it must
// leave alone, so the row catches a missed finding and a false one.
type m3Case struct {
	check     string
	red, near func(repo string) error
}

// m3Cases answers the six cases by name. Each near edit differs from its
// red edit in the one fact the check decides on: a linter outside the
// required set, an exclusion golaw.yml allows, an included file, an
// optional include, a recorded module and an allowlist of regexes.
func m3Cases() map[string]m3Case {
	return map[string]m3Case{
		"disabled-linter":       {check: m3GolangciCheck, red: m3Disable("errcheck"), near: m3Disable("lll")},
		"hollowed-by-exclusion": {check: m3GolangciCheck, red: m3Exclude("internal/", "errcheck"), near: m3Exclude(`_test\.go`, "mnd")},
		"modular-makefile":      {check: m3MakefileCheck, red: m3MoveRule(m3MovedTarget, false), near: m3MoveRule(m3MovedTarget, true)},
		"missing-include": {
			check: m3MakefileCheck,
			red:   m3Append(m3Makefile, "include mk/absent.mk\n"), near: m3Append(m3Makefile, "-include mk/absent.mk\n"),
		},
		"second-require-block": {check: "policy.dependency-records", red: m3SecondBlock(false), near: m3SecondBlock(true)},
		"generated-file-allowlist": {
			check: "policy.gitleaks-generic",
			red:   m3Append(m3GitleaksFile, "\n[allowlist]\npaths = ['''^gen/''']\n"),
			near:  m3Append(m3GitleaksFile, "\n[allowlist]\nregexes = ['''^EXAMPLE-''']\n"),
		},
	}
}

func m3CaseNames() []string {
	return slices.Sorted(maps.Keys(m3Cases()))
}

// caseM3 adopts the service once and judges a copy per side, so the red
// and the near edit start from the same tree and neither sees the other.
func (s *suite) caseM3(ctx context.Context, name string) error {
	named, found := m3Cases()[name]
	if !found {
		return fmt.Errorf("unknown M3 case %q: %w", name, ErrAcceptance)
	}
	repo, err := s.m3ServiceRepository(ctx, name, []string{"go"})
	if err != nil {
		return err
	}
	if err := s.m3CaseSide(ctx, repo, redWord, named.check, named.red); err != nil {
		return fmt.Errorf("the red side of %s: %w", name, err)
	}
	if err := s.m3CaseSide(ctx, repo, "near", named.check, named.near); err != nil {
		return fmt.Errorf("the near side of %s: %w", name, err)
	}
	return nil
}

// m3CaseSide copies the adopted service under the case's base, edits the
// copy and judges its scan; the side names the copy's directory.
func (s *suite) m3CaseSide(ctx context.Context, repo, side, check string, edit func(string) error) error {
	tree := filepath.Join(s.base, side)
	if err := m3CopyTree(repo, tree); err != nil {
		return err
	}
	if err := edit(tree); err != nil {
		return err
	}
	doc, code, err := s.m3Scan(ctx, tree)
	if err != nil {
		return err
	}
	return m3JudgeCase(check, side == redWord, doc, code)
}

// m3JudgeCase requires the check measured on both sides, because a stated
// limit proves neither a finding nor its absence. The red side must hold an
// enforced finding of the check at exit 1, the code of findings alone, so a
// scan that also left a check unmeasured never passes for the finding; the
// near side must hold no finding of it.
func m3JudgeCase(check string, red bool, doc scanreport.Document, code int) error {
	if status := m3Status(doc, check); status.Status != model.StatusMeasured.String() {
		return fmt.Errorf("%s is %q after the edit, want measured: %s: %w", check, status.Status, status.Reason, ErrAcceptance)
	}
	found := m3Enforced(doc, check)
	if red && (found == 0 || code != 1) {
		return fmt.Errorf("the scan exited %d with %d findings of %s, want exit 1 with a finding: %w", code, found, check, ErrAcceptance)
	}
	if !red && found > 0 {
		return fmt.Errorf("the near edit drew %d findings of %s, want none: %w", found, check, ErrAcceptance)
	}
	return nil
}

// m3Disable appends a linter to linters.disable of the lint configuration:
// a linter switched off by name, which the check must tell apart from one
// the strict asset never enables.
func m3Disable(linter string) func(string) error {
	return func(repo string) error {
		return m3EditYAML(filepath.Join(repo, m3LintFile), func(root *yaml.Node) error {
			disable, err := m3Child(root, yaml.SequenceNode, m3LintersKey, "disable")
			if err != nil {
				return err
			}
			m3AppendItem(disable, m3Scalar(linter))
			return nil
		})
	}
}

// m3Exclude appends an exclusion rule of one path and one linter to
// linters.exclusions.rules, the shape a hollowing exclusion takes.
func m3Exclude(path, linter string) func(string) error {
	return func(repo string) error {
		return m3EditYAML(filepath.Join(repo, m3LintFile), func(root *yaml.Node) error {
			rules, err := m3Child(root, yaml.SequenceNode, m3LintersKey, "exclusions", rulesWord)
			if err != nil {
				return err
			}
			linters := &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{m3Scalar(linter)}}
			m3AppendItem(rules, &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
				m3Scalar("path"), m3Scalar(path), m3Scalar(m3LintersKey), linters,
			}})
			return nil
		})
	}
}

// m3Child walks mapping keys from the root and creates each one that is
// absent, the last with the kind asked; a key that holds another kind is
// refused, because rewriting it would change what the file already says.
func m3Child(root *yaml.Node, kind yaml.Kind, keys ...string) (*yaml.Node, error) {
	node := root
	for i, key := range keys {
		want := yaml.MappingNode
		if i == len(keys)-1 {
			want = kind
		}
		next := m3Node(node, key)
		if next == nil {
			if node.Kind != yaml.MappingNode {
				return nil, fmt.Errorf("the lint configuration cannot hold %s: %w", strings.Join(keys[:i+1], "."), ErrAcceptance)
			}
			next = &yaml.Node{Kind: want}
			node.Content = append(node.Content, m3Scalar(key), next)
		}
		if next.Kind != want {
			return nil, fmt.Errorf("%s of the lint configuration is not the node the case edits: %w", strings.Join(keys[:i+1], "."), ErrAcceptance)
		}
		node = next
	}
	return node, nil
}

// m3AppendItem appends to a sequence and writes it in block style whatever
// style it was read in, so the edited entry sits on a line of its own.
func m3AppendItem(sequence, item *yaml.Node) {
	sequence.Style &^= yaml.FlowStyle
	sequence.Content = append(sequence.Content, item)
}

// m3Scalar tags its value as a string, so the encoder quotes a value YAML
// would otherwise read as another type.
func m3Scalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

// m3MoveRule cuts the rule of a target from golaw.mk into its own make file,
// included from the Makefile or not: the modular Makefile a developer
// splits, which the check must follow through the include.
func m3MoveRule(target string, included bool) func(string) error { //nolint:unparam // every editor names what it edits; the one modular case moves deadcode
	return func(repo string) error {
		body, err := readFile(filepath.Join(repo, m2EditedAsset))
		if err != nil {
			return err
		}
		lines := strings.SplitAfter(string(body), "\n")
		start := m3RuleLine(lines, target)
		if start < 0 {
			return fmt.Errorf("%s holds no rule of %s to move: %w", m2EditedAsset, target, ErrAcceptance)
		}
		end := start + 1
		for end < len(lines) && strings.HasPrefix(lines[end], "\t") {
			end++
		}
		rule := strings.Join(lines[start:end], "")
		if err := writeFile(filepath.Join(repo, filepath.FromSlash(m3QualityFile)), []byte(rule), fileMode); err != nil {
			return err
		}
		if err := m3RemoveRule(m2EditedAsset, target)(repo); err != nil {
			return err
		}
		if !included {
			return nil
		}
		return m3Append(m3Makefile, "include "+m3QualityFile+"\n")(repo)
	}
}

// m3SecondBlock appends a second require block to go.mod and, when the case
// records it, the decision that names its module: the reader must judge
// every block, not the first one alone.
func m3SecondBlock(recorded bool) func(string) error {
	return func(repo string) error {
		if err := m3Append(moduleFile, "\nrequire (\n\t"+m3SecondModule+" v1.0.0\n)\n")(repo); err != nil {
			return err
		}
		if !recorded {
			return nil
		}
		return writeFile(filepath.Join(repo, filepath.FromSlash(m3SecondRecord)), []byte(m3SecondRecordText), fileMode)
	}
}
