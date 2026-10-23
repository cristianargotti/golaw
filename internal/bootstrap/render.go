package bootstrap

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// capabilityGo is the capability every Go module gets, so the Go checks
// apply without the person naming the language of the repository.
const capabilityGo = "go"

// languagePattern is the grammar of the human channel language: a two or
// three letter language, optionally with a region (pt-BR, en, en-US).
var languagePattern = regexp.MustCompile(`^[a-z]{2,3}(-[A-Z]{2})?$`)

// policyTemplate is the policy file of a core-only repository with every
// floor at its default (specification section 5.3). The comment at the top
// travels with the file so a reviewer of the first commit reads the rule.
const policyTemplate = `# golaw policy, written by golaw init. Every later change is a reviewed
# diff that may only tighten the policy in force: floors fall, sets shrink,
# nothing here weakens a rule (golaw.yml is read from HEAD, never from the
# working tree).
golaw: 1
kit: { version: {{.KitVersion}} }
{{if .Profile}}{{pin .Profile}}
{{end}}language: {{.Language}}
capabilities: [{{join .Capabilities ", "}}]
layout: { main: {{.Layout.Main}}, decisions: {{.Layout.Decisions}} }
checks:
  sizes: { file_lines: 300, function_lines: 40 }
  coverage: { total: 80, package: 80, file: 80, min_statements: 20, file_exemptions: [] }
  terms: { tools: [], allow_paths: [] }
  lint:
    allowed_exclusions:
      - path: _test\.go
        linters: [funlen, mnd, goconst, dupl, gosec, noctx, bodyclose, gochecknoglobals, musttag]
  shell: { deny_push: false, deny_pr: false }
ladder: { pre_commit: true, commit_msg: true, pre_push: true, test_staged: false }
vulnerability_waivers: []
exceptions: []
managed: []
`

// Inputs are the typed values the policy file is rendered from.
type Inputs struct {
	KitVersion   string             // the version of the binary, the exact pin of A10
	Language     string             // the human channel language, pt-BR unless given
	Capabilities []string           // the capabilities to enable
	Layout       Layout             // what Detect found
	Profile      *policy.ProfileRef // the installed bundle the policy pins (A19), nil for core only
}

// Normalize validates the inputs and returns the canonical form the
// template renders: a known language, capabilities known to the model,
// sorted and unique, with go added for a Go module.
func (in Inputs) Normalize() (Inputs, error) {
	if in.KitVersion == "" {
		return Inputs{}, fmt.Errorf("%w: init needs the kit version", model.ErrUsage)
	}
	if !languagePattern.MatchString(in.Language) {
		return Inputs{}, fmt.Errorf("%w: language %q is not a tag like pt-BR or en", model.ErrUsage, in.Language)
	}
	if in.Layout.Main == "" || in.Layout.Decisions == "" {
		return Inputs{}, fmt.Errorf("%w: init needs a detected layout", model.ErrUsage)
	}
	if p := in.Profile; p != nil && (p.Name == "" || p.Source == "" || p.Version == "" || p.Digest == "") {
		return Inputs{}, fmt.Errorf("%w: the profile pin needs a name, a source, a version and a digest", model.ErrUsage)
	}
	capabilities := slices.Clone(in.Capabilities)
	if in.Layout.Go {
		capabilities = append(capabilities, capabilityGo)
	}
	slices.Sort(capabilities)
	capabilities = slices.Compact(capabilities)
	for _, name := range capabilities {
		if !slices.Contains(model.Capabilities(), name) {
			return Inputs{}, fmt.Errorf("%w: unknown capability %q", model.ErrUsage, name)
		}
	}
	if err := policy.CheckCapabilities(capabilities); err != nil {
		return Inputs{}, fmt.Errorf("%w: %w", model.ErrUsage, err)
	}
	in.Capabilities = capabilities
	return in, nil
}

// Render writes the policy text for normalized inputs; the same inputs
// render the same bytes, which is what makes init idempotent.
func Render(in Inputs) ([]byte, error) {
	funcs := template.FuncMap{
		"join": strings.Join,
		"pin":  func(ref *policy.ProfileRef) string { return renderProfileLine(*ref) },
	}
	tmpl, err := template.New(PolicyFile).Funcs(funcs).Parse(policyTemplate)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: parse template: %w", err)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, in); err != nil {
		return nil, fmt.Errorf("bootstrap: render %s: %w", PolicyFile, err)
	}
	return out.Bytes(), nil
}
