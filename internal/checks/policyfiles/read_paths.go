package policyfiles

import (
	"slices"
	"strings"
)

// The configuration files a reader of the family may read: at the root of
// the repository, and below it, beside the make includes, the Dockerfiles,
// the Actions workflows and the decision records.
const (
	rootConfigurations = "Makefile GNUmakefile makefile go.mod go.work lefthook.yml lefthook.yaml lefthook-local.yml " +
		".golangci.yml .golangci.yaml .golangci.toml .golangci.json .gitleaks.toml .squawk.toml CODEOWNERS " +
		"sqlc.yaml sqlc.yml sqlc.json sqlc.vet.yaml redocly.yaml redocly.yml .redocly.yaml .redocly.yml .redocly.lint-ignore.yaml"
	nestedConfigurations = "tools/go.mod scripts/lint/rules.go docs/CODEOWNERS .github/CODEOWNERS " +
		".github/dependabot.yml .github/dependabot.yaml"
	recordExtension = ".md"
)

// configured answers whether a path is a configuration file a reader of the
// family may read, for the decision records under the directory given.
func configured(decisions string) func(string) bool {
	directory := strings.TrimSuffix(decisions, "/") + "/"
	return func(name string) bool {
		switch {
		case strings.HasSuffix(name, ".mk") || isDockerfile(name) || strings.HasPrefix(name, actionsDir):
			return true
		case strings.HasPrefix(name, directory):
			return strings.HasSuffix(strings.ToLower(name), recordExtension)
		case strings.Contains(name, "/"):
			return slices.Contains(strings.Fields(nestedConfigurations), name)
		}
		return slices.Contains(strings.Fields(rootConfigurations), name)
	}
}
