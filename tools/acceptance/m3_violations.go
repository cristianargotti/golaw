package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
	scanreport "github.com/cristianargotti/golaw/internal/report"
)

// m3Violation is one row of the policy category: the check it proves, the
// capabilities the service adopts under, and the smallest edit that makes
// the check red on the rendered service.
type m3Violation struct {
	check        string
	capabilities []string
	apply        func(repo string) error
}

// m3Violations answers the violation rows by name, one per policy check
// that applies to the rendered service; the two profile checks have a row
// of their own, because without a profile they apply nowhere (D11).
func m3Violations() map[string]m3Violation {
	goOnly := []string{"go"}
	return map[string]m3Violation{
		"golangci":            {check: "policy.golangci", capabilities: goOnly, apply: m3DisableShadow},
		"ruleguard":           {check: "policy.ruleguard", capabilities: goOnly, apply: m3Remove(m3RulesFile)},
		"gitleaks-generic":    {check: "policy.gitleaks-generic", capabilities: goOnly, apply: m3Append(".gitleaks.toml", "\n[allowlist]\npaths = ['''^gen/''']\n")},
		"gotoolchain-local":   {check: "policy.gotoolchain-local", capabilities: goOnly, apply: m3Replace(m2EditedAsset, "export GOTOOLCHAIN=local\n", "")},
		"makefile":            {check: "policy.makefile", capabilities: goOnly, apply: m3RemoveRule(m2EditedAsset, "deadcode")},
		"ci-target":           {check: "policy.ci-target", capabilities: goOnly, apply: m3RemovePrerequisite(m2EditedAsset, "ci", "deadcode")},
		"generated-fresh":     {check: "policy.generated-fresh", capabilities: goOnly, apply: m3RemoveRule(m2EditedAsset, "generate-check")},
		"dependency-records":  {check: "policy.dependency-records", capabilities: goOnly, apply: m3Append(moduleFile, "\nrequire example.test/unrecorded v1.0.0\n")},
		"dependabot-cooldown": {check: "policy.dependabot-cooldown", capabilities: goOnly, apply: m3Replace(".github/dependabot.yml", "default-days: 7", "default-days: 1")},
		"deps-age-config":     {check: "policy.deps-age-config", capabilities: goOnly, apply: m3Replace(m2EditedAsset, "-days 7", "-days 1")},
		"image-digest":        {check: "policy.image-digest", capabilities: []string{"go", m3CapabilityImage}, apply: m3DropDigest},
		"openapi-assertions":  {check: "policy.openapi-assertions", capabilities: []string{m3CapabilityAPI, "go"}, apply: m3Remove("redocly.yaml")},
		"squawk-config":       {check: "policy.squawk-config", capabilities: []string{"db", "go"}, apply: m3Remove(".squawk.toml")},
	}
}

// m3Edit rewrites one file of a repository through an edit of its text;
// the read refuses an absent file and an edit that refuses writes nothing,
// so a row never judges a file it created or half changed.
func m3Edit(repo, file string, edit func(string) (string, error)) error {
	path := filepath.Join(repo, filepath.FromSlash(file))
	body, err := readFile(path)
	if err != nil {
		return err
	}
	text, err := edit(string(body))
	if err != nil {
		return err
	}
	return writeFile(path, []byte(text), fileMode)
}

// m3Replace replaces the first occurrence of a text in a file and refuses
// a file that does not hold it, because a row over an unchanged tree would
// prove nothing.
func m3Replace(file, old, replacement string) func(string) error {
	return func(repo string) error {
		return m3Edit(repo, file, func(text string) (string, error) {
			if !strings.Contains(text, old) {
				return "", fmt.Errorf("%s does not hold %q: %w", file, old, ErrAcceptance)
			}
			return strings.Replace(text, old, replacement, 1), nil
		})
	}
}

// m3Append adds a text at the end of a file, where a TOML table or a go.mod
// block takes effect without rewriting what the rendered file holds.
func m3Append(file, text string) func(string) error {
	return func(repo string) error {
		return m3Edit(repo, file, func(current string) (string, error) { return current + text, nil })
	}
}

// m3Remove deletes a file of the repository and refuses an absent one.
func m3Remove(file string) func(string) error {
	return func(repo string) error {
		if err := os.Remove(filepath.Join(repo, filepath.FromSlash(file))); err != nil {
			return fmt.Errorf("remove %s: %w", file, errors.Join(ErrAcceptance, err))
		}
		return nil
	}
}

// m3RuleLine is the index of the rule line of a make target: the line that
// starts with the target and a colon and assigns no variable, which a
// target-specific assignment of the same target would.
func m3RuleLine(lines []string, target string) int {
	return slices.IndexFunc(lines, func(line string) bool {
		rest, found := strings.CutPrefix(strings.TrimRight(line, "\r\n"), target+":")
		return found && !strings.Contains(rest, "=")
	})
}

// m3RemoveRule drops the rule of a make target: its rule line, the recipe
// lines after it and the one blank line that follows them.
func m3RemoveRule(file, target string) func(string) error { //nolint:unparam // every editor names its file; both rules the rows remove happen to live in golaw.mk
	return func(repo string) error {
		return m3Edit(repo, file, func(text string) (string, error) {
			lines := strings.SplitAfter(text, "\n")
			start := m3RuleLine(lines, target)
			if start < 0 {
				return "", fmt.Errorf("%s has no rule for %s: %w", file, target, ErrAcceptance)
			}
			end := start + 1
			for end < len(lines) && strings.HasPrefix(lines[end], "\t") {
				end++
			}
			if end < len(lines) && strings.TrimRight(lines[end], "\r\n") == "" {
				end++
			}
			return strings.Join(slices.Delete(lines, start, end), ""), nil
		})
	}
}

// m3RemovePrerequisite drops one word from the prerequisites of the rule
// line of a make target.
func m3RemovePrerequisite(file, target, prerequisite string) func(string) error {
	return func(repo string) error {
		return m3Edit(repo, file, func(text string) (string, error) {
			lines := strings.SplitAfter(text, "\n")
			index := m3RuleLine(lines, target)
			if index < 0 {
				return "", fmt.Errorf("%s has no rule for %s: %w", file, target, ErrAcceptance)
			}
			head, rest, _ := strings.Cut(lines[index], ":")
			words := strings.Fields(rest)
			at := slices.Index(words, prerequisite)
			if at < 0 {
				return "", fmt.Errorf("%s: the rule %s has no prerequisite %s: %w", file, target, prerequisite, ErrAcceptance)
			}
			lines[index] = head + ": " + strings.Join(slices.Delete(words, at, at+1), " ") + "\n"
			return strings.Join(lines, ""), nil
		})
	}
}

// m3EditYAML decodes a YAML file into its node tree, edits the root mapping
// and writes the tree back, so an edit keeps every other key and comment.
func m3EditYAML(path string, edit func(*yaml.Node) error) error {
	body, err := readFile(path)
	if err != nil {
		return err
	}
	var document yaml.Node
	if err := yaml.Unmarshal(body, &document); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return fmt.Errorf("%s is not one YAML document: %w", path, ErrAcceptance)
	}
	if err := edit(document.Content[0]); err != nil {
		return err
	}
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := errors.Join(encoder.Encode(&document), encoder.Close()); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return writeFile(path, out.Bytes(), fileMode)
}

// m3Node walks mapping keys from a node and answers nil when one is absent,
// so the caller names the missing key in its own refusal.
func m3Node(node *yaml.Node, keys ...string) *yaml.Node {
	for _, key := range keys {
		if node == nil || node.Kind != yaml.MappingNode {
			return nil
		}
		var next *yaml.Node
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				next = node.Content[i+1]
			}
		}
		node = next
	}
	return node
}

// m3DisableShadow appends shadow to the disabled govet analyzers of the
// rendered lint configuration: the smallest edit that drops an analyzer the
// policy requires while the file still parses.
func m3DisableShadow(repo string) error {
	return m3EditYAML(filepath.Join(repo, ".golangci.yml"), func(root *yaml.Node) error {
		disable := m3Node(root, "linters", "settings", "govet", "disable")
		if disable == nil || disable.Kind != yaml.SequenceNode {
			return fmt.Errorf("the lint configuration has no govet disable list: %w", ErrAcceptance)
		}
		disable.Content = append(disable.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "shadow"})
		return nil
	})
}

// m3DropDigest removes the first image digest of the Dockerfile, which
// leaves that image by tag, the reference policy.image-digest refuses.
func m3DropDigest(repo string) error {
	return m3Edit(repo, "Dockerfile", func(text string) (string, error) {
		at := regexp.MustCompile(`@sha256:[0-9a-f]{64}`).FindStringIndex(text)
		if at == nil {
			return "", fmt.Errorf("the Dockerfile holds no image digest: %w", ErrAcceptance)
		}
		return text[:at[0]] + text[at[1]:], nil
	})
}

// m3Status answers the status of a check in a scan document, an empty one
// when the document does not name the check, so an absent check fails a
// judge as an unmeasured one does.
func m3Status(doc scanreport.Document, check string) scanreport.Status {
	for _, status := range doc.Checks {
		if status.ID == check {
			return status
		}
	}
	return scanreport.Status{}
}

// m3Enforced counts the enforced findings of a check in a scan document;
// a note leaves make ci green, so only an enforced finding makes the
// service unclean.
func m3Enforced(doc scanreport.Document, check string) int {
	count := 0
	for _, finding := range doc.Findings {
		if finding.Check == check && finding.Enforced() {
			count++
		}
	}
	return count
}

// m3JudgeClean requires a check measured and without a finding on the
// rendered service, so the failure that follows is the violation's.
func m3JudgeClean(doc scanreport.Document, check string) error {
	if status := m3Status(doc, check); status.Status != model.StatusMeasured.String() {
		return fmt.Errorf("%s is %q on the clean service, want measured: %s: %w", check, status.Status, status.Reason, ErrAcceptance)
	}
	if found := m3Enforced(doc, check); found > 0 {
		return fmt.Errorf("%s has %d findings on the clean service: %w", check, found, ErrAcceptance)
	}
	return nil
}

// m3JudgeViolation requires a started make, a nonzero exit and the check
// named in the output: the proof that the scan target failed the CI on the
// violation and nothing else.
func m3JudgeViolation(check string, got result) error {
	if got.err != nil {
		return fmt.Errorf("execute make ci under the %s violation: %w", check, got.err)
	}
	if got.code == 0 {
		return fmt.Errorf("make ci passed under the %s violation: %s: %w", check, m3Tail(got), ErrAcceptance)
	}
	if !strings.Contains(got.stdout, check) && !strings.Contains(got.stderr, check) {
		return fmt.Errorf("make ci failed without naming %s: %s: %w", check, m3Tail(got), ErrAcceptance)
	}
	return nil
}
