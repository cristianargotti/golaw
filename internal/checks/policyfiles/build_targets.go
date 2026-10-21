package policyfiles

import "slices"

const (
	buildAPICapability   = "api"
	buildImageCapability = "image"
)

// buildTarget is one make target the ci target carries and the capability
// that requires it; an empty capability holds for every repository.
type buildTarget struct {
	name, capability string
}

// RequiredTargets names the make targets the ci target of a repository must
// carry under its capabilities, in the order of the ladder table.
func RequiredTargets(capabilities []string) []string {
	var targets []string
	for _, target := range buildTargets() {
		if target.capability == "" || slices.Contains(capabilities, target.capability) {
			targets = append(targets, target.name)
		}
	}
	return targets
}

// buildTargets is the ladder table of section 7 in its merged order, one
// capability per row. scan has no capability because golaw's scan carries
// the families every repository obeys and is the one target through which a
// violation fails the pipeline; hooks-test is profile data and stays out.
func buildTargets() []buildTarget {
	return []buildTarget{
		{"mod", "go"},
		{"deps-age", "go"},
		{"fmt", "go"},
		{"vet", "go"},
		{"generate-check", "go"},
		{"test", "go"},
		{"coverage-check", "go"},
		{"build", "go"},
		{"lint", "go"},
		{"deadcode", "go"},
		{"migrate-lint", "db"},
		{"openapi-breaking", buildAPICapability},
		{"openapi-examples-check", buildAPICapability},
		{"vuln", "go"},
		{"licenses", "go"},
		{"docs-check", buildAPICapability},
		{"secrets", "go"},
		{"scan", ""},
		{"dockerfile-lint", buildImageCapability},
		{"image-pins-check", buildImageCapability},
		{buildImageCapability, buildImageCapability},
		{"image-scan", buildImageCapability},
	}
}
