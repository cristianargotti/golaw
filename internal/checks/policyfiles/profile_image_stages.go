package policyfiles

import (
	"context"
	"fmt"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// platformDockerfile is the one Dockerfile the image rules of a profile
// judge: the image the platform builds sits at the root, and a Dockerfile
// elsewhere (a local one, a fixture) is never the platform's image.
const platformDockerfile = "Dockerfile"

type imageStages struct{}

// ImageStages is policy.image-stages: the named stages of the root
// Dockerfile in the order the profile gives them, each present, and the
// last stage built from the profile's final image (section 5.4).
func ImageStages() model.Check { return imageStages{} }

func (imageStages) Spec() model.Spec { return spec("image-stages", ruleProfile, buildImageCapability) }

func (imageStages) Run(_ context.Context, in model.Input) (model.Result, error) {
	if status, ok := section(in, model.SectionDockerfile); !ok {
		return model.Result{Status: status}, nil
	}
	rules := in.Policy.ProfileData.Dockerfile
	if len(rules.Stages) == 0 && rules.FinalImage == "" {
		return model.Result{Status: model.NotApplicable("the profile names no stage order and no final image")}, nil
	}
	document, found, err := rootDockerfile(in)
	switch {
	case err != nil:
		return answer(platformDockerfile, err), nil
	case !found:
		return noRootDockerfile(), nil
	}
	return judgedDockerfile(append(stageOrder(document, rules.Stages), finalImage(document, rules.FinalImage)...)), nil
}

// noRootDockerfile is the answer of an image rule over a tree without a
// root Dockerfile: measured, nothing to judge.
func noRootDockerfile() model.Result {
	return model.Result{Status: model.Measured(), Scope: "no Dockerfile at the root"}
}

// judgedDockerfile is the answer of an image rule over the root Dockerfile.
func judgedDockerfile(findings []model.Finding) model.Result {
	return model.Result{Status: model.Measured(), Scope: "1 Dockerfile", Findings: findings}
}

// rootDockerfile reads the platform's Dockerfile through the subset of
// readDockerfile; found is false when the tree holds none.
func rootDockerfile(in model.Input) (dockerfile, bool, error) {
	data, found, err := read(in, platformDockerfile)
	if err != nil || !found {
		return dockerfile{}, false, err
	}
	document, err := readDockerfile(platformDockerfile, data)
	return document, err == nil, err
}

// stageOrder reports a named stage that comes after a stage the profile
// orders later, and every stage of the profile the Dockerfile lacks. Stage
// names compare in lower case, as the builder resolves them.
func stageOrder(document dockerfile, order []string) []model.Finding {
	rank := make(map[string]int, len(order))
	for i, name := range order {
		rank[name] = i
	}
	listed := strings.Join(order, ", ")
	seen := make(map[string]bool, len(order))
	var findings []model.Finding
	last, lastName := -1, ""
	for _, stage := range document.stages {
		name := strings.ToLower(stage.name)
		position, ok := rank[name]
		if !ok || seen[name] {
			continue
		}
		seen[name] = true
		if position < last {
			message := fmt.Sprintf("stage %s of %s comes after %s: the profile orders the stages %s", name, document.path, lastName, listed)
			findings = append(findings, finding(document.path, stage.line, "image-stage-order", message))
			continue
		}
		last, lastName = position, name
	}
	for _, name := range order {
		if !seen[name] {
			message := fmt.Sprintf("add the stage %s to %s: the profile orders the stages %s", name, document.path, listed)
			findings = append(findings, finding(document.path, 0, "image-stage-missing", message))
		}
	}
	return findings
}

// finalImage reports a last stage built from another image than the one
// the profile names; an empty name leaves the final image free.
func finalImage(document dockerfile, image string) []model.Finding {
	if image == "" || len(document.stages) == 0 {
		return nil
	}
	at := len(document.stages) - 1
	from := stageImage(document.stages, at)
	if strings.EqualFold(from, image) {
		return nil
	}
	message := fmt.Sprintf("the last stage of %s starts from %s: build the final image from %s, as the profile requires", document.path, from, image)
	return []model.Finding{finding(document.path, document.stages[at].line, "image-final-image", message)}
}

// stageImage resolves the from of a stage through the names of the stages
// before it, the only ones it can name, to the image it is built from,
// because a stage that starts from an earlier stage carries that image.
// Names compare in lower case, as the builder resolves them.
func stageImage(stages []dockerStage, at int) string {
	from := stages[at].from
	for i := at - 1; i >= 0; i-- {
		if stages[i].name != "" && strings.EqualFold(stages[i].name, from) {
			from = stages[i].from
		}
	}
	return from
}
