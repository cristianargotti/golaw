package policyfiles

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

type imageInstalls struct{}

// ImageInstalls is policy.image-installs: in the root Dockerfile, every
// package an install line of a stage the profile lists is in that stage's
// allowlist, every installed package carries =version when the profile
// pins installs, and no comment line starts with a prefix the profile
// forbids, an inline linter ignore among them (section 5.4).
func ImageInstalls() model.Check { return imageInstalls{} }

func (imageInstalls) Spec() model.Spec {
	return spec("image-installs", ruleProfile, buildImageCapability)
}

func (imageInstalls) Run(_ context.Context, in model.Input) (model.Result, error) {
	if status, ok := section(in, model.SectionDockerfile); !ok {
		return model.Result{Status: status}, nil
	}
	rules := in.Policy.ProfileData.Dockerfile
	if len(rules.Installs) == 0 && !rules.PinnedInstalls && len(rules.IgnoreComments) == 0 {
		return model.Result{Status: model.NotApplicable("the profile names no install allowlist, no pinned installs and no forbidden comment")}, nil
	}
	data, found, err := read(in, platformDockerfile)
	switch {
	case err != nil:
		return answer(platformDockerfile, err), nil
	case !found:
		return noRootDockerfile(), nil
	}
	document, err := readDockerfile(platformDockerfile, data)
	if err != nil {
		return answer(platformDockerfile, err), nil
	}
	commands, err := readInstalls(platformDockerfile, data)
	if err != nil {
		return answer(platformDockerfile, err), nil
	}
	return judgedDockerfile(append(installFindings(document, commands, rules), commentFindings(commands, rules.IgnoreComments)...)), nil
}

// installFindings judges every installed package against the allowlist of
// its stage and the pin rule; a stage the profile does not list is free.
func installFindings(document dockerfile, commands imageCommands, rules *model.DockerfileRules) []model.Finding {
	var findings []model.Finding
	for _, p := range commands.installs {
		stage := stageAt(document, p.line)
		allowed := slices.IndexFunc(rules.Installs, func(a model.InstallAllowlist) bool { return a.Stage == stage })
		if allowed >= 0 && !slices.Contains(rules.Installs[allowed].Packages, p.name) {
			message := fmt.Sprintf("stage %s of %s installs %s: the profile allows only %s in that stage",
				stage, commands.path, p.name, strings.Join(rules.Installs[allowed].Packages, ", "))
			findings = append(findings, finding(commands.path, p.line, "image-install-not-allowed", message))
		}
		if rules.PinnedInstalls && p.version == "" {
			message := fmt.Sprintf("pin %s in %s as %s=<version>: the profile requires every installed package pinned", p.name, commands.path, p.name)
			findings = append(findings, finding(commands.path, p.line, "image-install-unpinned", message))
		}
	}
	return findings
}

// stageAt is the lower case name of the stage a line belongs to, empty for
// an unnamed stage or a line before the first FROM.
func stageAt(document dockerfile, line int) string {
	name := ""
	for _, stage := range document.stages {
		if stage.line < line {
			name = strings.ToLower(stage.name)
		}
	}
	return name
}

// commentFindings reports a comment line that starts with a forbidden
// prefix, spaces after the mark and between words not counted.
func commentFindings(commands imageCommands, prefixes []string) []model.Finding {
	var findings []model.Finding
	for _, c := range commands.comments {
		for _, prefix := range prefixes {
			if strings.HasPrefix(commentWords(c.text), commentWords(prefix)) {
				message := fmt.Sprintf("remove the comment %q from %s: the profile forbids %q, fix the rule it would silence", c.text, commands.path, prefix)
				findings = append(findings, finding(commands.path, c.line, "image-inline-ignore", message))
				break
			}
		}
	}
	return findings
}

// commentWords is a comment without its mark, in lower case, its words
// joined by one space.
func commentWords(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimPrefix(strings.TrimSpace(text), "#")), " "))
}
