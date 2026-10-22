package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
	scanreport "github.com/cristianargotti/golaw/internal/report"
)

const (
	m3CapabilityCategory = "m3-capability"
	m3APICapability      = "api"
	m3ImageCapability    = "image"
	m3DomainRule         = "domain"
	m3RawResponseFile    = "internal/server/raw.go"
	// m3RawResponse is the gin shape goast.raw-error-response reads by
	// name: a 5xx answered by a handler outside the errors package.
	m3RawResponse = "package server\n\n" +
		"type responder interface {\n\tJSON(status int, body any)\n}\n\n" +
		"func answerRaw(r responder) {\n\tr.JSON(500, map[string]string{\"error\": \"raw\"})\n}\n"
)

// m3CapabilityCase is one capability combination on the service: the
// checks it shapes, whether the capability conditions them or only shapes
// a rule inside one, and the violation applied to the tree with the
// capability and to the tree without it.
type m3CapabilityCase struct {
	checks      []string
	conditioned bool
	apply       func(repo string) error
}

// m3CapabilityCases answers the combinations section 11 names. The first
// check of each is the one the violation must reach; db and hexagonal edit
// only what the tree with the capability holds, so both trees end alike.
func m3CapabilityCases() map[string]m3CapabilityCase {
	return map[string]m3CapabilityCase{
		m3APICapability: {
			checks: []string{"goast.raw-error-response", "policy.openapi-assertions"}, conditioned: true,
			apply: m3WriteRawResponse,
		},
		"db":              {checks: []string{"policy.squawk-config"}, conditioned: true, apply: m3RemovePresent(".squawk.toml")},
		"hexagonal":       {checks: []string{m3GolangciCheck}, apply: m3DropDomainRule},
		m3ImageCapability: {checks: []string{"policy.image-digest"}, conditioned: true, apply: m3DropDigests},
	}
}

func m3CapabilityNames() []string {
	return slices.Sorted(maps.Keys(m3CapabilityCases()))
}

// capabilityM3 renders the service with the capability and without it,
// proves the tree with it clean, applies the same violation to both and
// judges the two scans, so the only difference the row measures is the
// capability.
func (s *suite) capabilityM3(ctx context.Context, name string) error {
	combination, found := m3CapabilityCases()[name]
	if !found {
		return fmt.Errorf("unknown M3 capability combination %q: %w", name, ErrAcceptance)
	}
	capabilities := []string{"go", name}
	slices.Sort(capabilities)
	with, err := s.m3CleanService(ctx, "with", capabilities, combination.checks)
	if err != nil {
		return err
	}
	without, err := s.m3ServiceRepository(ctx, "without", []string{"go"})
	if err != nil {
		return err
	}
	docs := make([]scanreport.Document, 0, 2)
	for _, repo := range []string{with, without} {
		if err := combination.apply(repo); err != nil {
			return err
		}
		doc, _, err := s.m3Scan(ctx, repo)
		if err != nil {
			return err
		}
		docs = append(docs, doc)
	}
	return m3JudgeCapability(name, combination, docs[0], docs[1])
}

// m3CleanService adopts the service under the capabilities and requires
// every check of the combination measured without a finding, so a finding
// after the violation is the violation's.
func (s *suite) m3CleanService(ctx context.Context, name string, capabilities, checks []string) (string, error) {
	repo, err := s.m3ServiceRepository(ctx, name, capabilities)
	if err != nil {
		return "", err
	}
	doc, code, err := s.m3Scan(ctx, repo)
	if err != nil {
		return "", err
	}
	for _, check := range checks {
		if err := m3JudgeClean(doc, check); err != nil {
			return "", fmt.Errorf("the scan under %s exited %d: %w", strings.Join(capabilities, ", "), code, err)
		}
	}
	return repo, nil
}

// m3JudgeCapability requires a finding of the first check with the
// capability; without it, a conditioned check must not apply and its
// reason must name the capability, and a shaped rule must leave its check
// measured without the finding.
func m3JudgeCapability(name string, c m3CapabilityCase, with, without scanreport.Document) error {
	if m3Enforced(with, c.checks[0]) == 0 {
		return fmt.Errorf("with %s the violation draws no finding of %s: %w", name, c.checks[0], ErrAcceptance)
	}
	if !c.conditioned {
		status := m3Status(without, c.checks[0])
		if status.Status != model.StatusMeasured.String() || m3Enforced(without, c.checks[0]) > 0 {
			return fmt.Errorf("without %s, %s is %q with %d findings, want measured without one: %w",
				name, c.checks[0], status.Status, m3Enforced(without, c.checks[0]), ErrAcceptance)
		}
		return nil
	}
	for _, check := range c.checks {
		status := m3Status(without, check)
		if status.Status != model.StatusNotApplicable.String() || !strings.Contains(status.Reason, name) {
			return fmt.Errorf("without %s, %s is %q (%s), want not applicable naming %s: %w",
				name, check, status.Status, status.Reason, name, ErrAcceptance)
		}
	}
	return nil
}

func m3WriteRawResponse(repo string) error {
	return writeFile(filepath.Join(repo, filepath.FromSlash(m3RawResponseFile)), []byte(m3RawResponse), fileMode)
}

// m3RemovePresent deletes a file only the tree with the capability holds;
// its absence from the other tree is the state the violation leaves.
func m3RemovePresent(file string) func(string) error {
	return func(repo string) error {
		err := os.Remove(filepath.Join(repo, filepath.FromSlash(file)))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", file, err)
		}
		return nil
	}
}

// m3DropDomainRule deletes the depguard rule that holds the domain layer,
// which only the tree with hexagonal renders.
func m3DropDomainRule(repo string) error {
	return m3EditYAML(filepath.Join(repo, m3LintFile), func(root *yaml.Node) error {
		rules := m3Node(root, m3LintersKey, "settings", "depguard", "rules")
		if rules == nil || rules.Kind != yaml.MappingNode {
			return fmt.Errorf("the lint configuration has no depguard rules: %w", ErrAcceptance)
		}
		for i := 0; i+1 < len(rules.Content); i += 2 {
			if rules.Content[i].Value == m3DomainRule {
				rules.Content = slices.Delete(rules.Content, i, i+2)
				break
			}
		}
		return nil
	})
}

// m3DropDigests leaves every image of the Dockerfile by tag: the service
// pins a build and a runtime image, and a digest left on either would keep
// that stage pinned.
func m3DropDigests(repo string) error {
	return m3Edit(repo, "Dockerfile", func(text string) (string, error) {
		digest := regexp.MustCompile(`@sha256:[0-9a-f]{64}`)
		if !digest.MatchString(text) {
			return "", fmt.Errorf("the Dockerfile holds no image digest: %w", ErrAcceptance)
		}
		return digest.ReplaceAllString(text, ""), nil
	})
}
