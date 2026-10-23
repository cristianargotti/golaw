package policyfiles

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

type imageRegistry struct{}

// ImageRegistry is policy.image-registry: every image reference a FROM
// line of the root Dockerfile pulls, after the ARG defaults are
// substituted, matches the pattern the profile names (section 5.4);
// scratch and a stage named before are never judged.
func ImageRegistry() model.Check { return imageRegistry{} }

func (imageRegistry) Spec() model.Spec {
	return spec("image-registry", ruleProfile, buildImageCapability)
}

func (imageRegistry) Run(_ context.Context, in model.Input) (model.Result, error) {
	if status, ok := section(in, model.SectionRegistry); !ok {
		return model.Result{Status: status}, nil
	}
	pattern, err := regexp.Compile(in.Policy.ProfileData.Registry.Pattern)
	if err != nil {
		return answer(platformDockerfile, parseError("registry.pattern of the profile", err)), nil
	}
	document, found, err := rootDockerfile(in)
	switch {
	case err != nil:
		return answer(platformDockerfile, err), nil
	case !found:
		return noRootDockerfile(), nil
	}
	return judgedDockerfile(registryFindings(document, pattern)), nil
}

// registryFindings reports every pulled image outside the pattern; a stage
// name counts once it is declared, as the builder resolves it.
func registryFindings(document dockerfile, pattern *regexp.Regexp) []model.Finding {
	aliases := make(map[string]bool)
	var findings []model.Finding
	for _, stage := range document.stages {
		from := strings.ToLower(stage.from)
		if from != "scratch" && !aliases[from] && !pattern.MatchString(stage.from) {
			message := fmt.Sprintf("base image %s of %s is outside the registry the profile names (%s): pull it from that registry",
				stage.from, document.path, pattern)
			findings = append(findings, finding(document.path, stage.line, "image-registry", message))
		}
		if stage.name != "" {
			aliases[strings.ToLower(stage.name)] = true
		}
	}
	return findings
}
