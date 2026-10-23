package policyfiles

import "strings"

// The configuration files a reader of the family may read: at the root of
// the repository, and below it, beside the make includes, the Dockerfiles,
// the Actions files and the decision records.
const (
	rootConfigurations = "Makefile GNUmakefile makefile go.mod go.work lefthook.yml lefthook.yaml lefthook-local.yml " +
		".golangci.yml .golangci.yaml .golangci.toml .golangci.json .gitleaks.toml .squawk.toml CODEOWNERS " +
		"sqlc.yaml sqlc.yml sqlc.json sqlc.vet.yaml redocly.yaml redocly.yml .redocly.yaml .redocly.yml .redocly.lint-ignore.yaml"
	nestedConfigurations = "tools/go.mod scripts/lint/rules.go docs/CODEOWNERS .github/CODEOWNERS " +
		".github/dependabot.yml .github/dependabot.yaml"
	recordExtension = ".md"
)

// configured answers whether a path is a configuration file a reader of the
// family may read, for the decision records under the directory given. The
// name lists are split once per predicate, not once per path, because the
// prefetch asks the predicate about every path of the tree.
func configured(decisions string) func(string) bool {
	directory := strings.TrimSuffix(decisions, "/") + "/"
	root, nested := nameSet(rootConfigurations), nameSet(nestedConfigurations)
	return func(name string) bool {
		switch {
		case strings.HasSuffix(name, ".mk") || isDockerfile(name) || strings.HasPrefix(name, actionsDir):
			return true
		case strings.HasPrefix(name, directory):
			return strings.HasSuffix(strings.ToLower(name), recordExtension)
		case strings.Contains(name, "/"):
			return nested[name]
		}
		return root[name]
	}
}

// nameSet holds the words of a list as a set.
func nameSet(list string) map[string]bool {
	words := strings.Fields(list)
	set := make(map[string]bool, len(words))
	for _, word := range words {
		set[word] = true
	}
	return set
}
