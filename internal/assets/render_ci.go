package assets

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"text/template"
)

// The pipeline resolves the enrollment and runner expressions when the CI
// file runs, so the CI template alone may hand them on unrendered; any other
// expression, or these in another asset, is still an unresolved placeholder.
const (
	ciSource      = "templates/.github/workflows/ci.yml.tmpl"
	ciDestination = ".github/workflows/ci.yml"
	ciEnrollment  = "${{ vars.GOLAW_ENROLLED }}"
	ciRunnerOS    = "${{ runner.os }}"
)

func executeAsset(text string, c Context, allowCI bool) ([]byte, error) {
	functions := template.FuncMap{"has": slices.Contains[[]string, string], "hasPrefix": strings.HasPrefix}
	parsed, err := template.New("asset").Funcs(functions).Option("missingkey=error").Parse(text)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	for _, item := range parsed.Templates() {
		if err := validateReferences(item.Root, c, ""); err != nil {
			return nil, err
		}
	}
	var output bytes.Buffer
	if err := parsed.Execute(&output, c); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}
	data := output.Bytes()
	if unresolvedAsset(data, allowCI) {
		return nil, errDelimiter
	}
	return data, nil
}

// The check reads a copy, so the expression reaches the output byte for byte.
func unresolvedAsset(data []byte, allowCI bool) bool {
	if allowCI {
		data = bytes.ReplaceAll(data, []byte(ciEnrollment), nil)
		data = bytes.ReplaceAll(data, []byte(ciRunnerOS), nil)
	}
	return bytes.Contains(data, []byte("{{")) || bytes.Contains(data, []byte("}}"))
}
