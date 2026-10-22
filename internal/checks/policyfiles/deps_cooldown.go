package policyfiles

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	cooldownDefault = "default-days"
	cooldownMajor   = "semver-major-days"
	cooldownMinor   = "semver-minor-days"
	cooldownPatch   = "semver-patch-days"
	cooldownDays    = 7
	cooldownMajors  = 30
)

type dependabotCooldown struct{}

// DependabotCooldown prevents overrides and package filters from bypassing
// the required update age while retaining the platform's security exemption.
func DependabotCooldown() model.Check { return dependabotCooldown{} }

func (dependabotCooldown) Spec() model.Spec { return spec("dependabot-cooldown", ruleSupply) }

func (dependabotCooldown) Run(_ context.Context, in model.Input) (model.Result, error) {
	file, data, found, err := dependabotFile(in)
	if err != nil {
		return answer(file, err), nil
	}
	if !found {
		reason := "no Dependabot configuration at .github/dependabot.yml; " +
			"init writes it unless CODEOWNERS gives .github/ to another team"
		return model.Result{Status: model.NotApplicable(reason)}, nil
	}
	configuration, err := readDependabot(file, data)
	if err != nil {
		return answer(file, err), nil
	}
	result := model.Result{Status: model.Measured(), Scope: file + "; cooldowns: gomod, github-actions"}
	for _, update := range configuration.updates {
		if repairs := cooldownRepairs(update); len(repairs) != 0 {
			message := cooldownMessage(file, update.ecosystem, repairs)
			result.Findings = append(result.Findings, finding(file, update.line, "dependabot-cooldown-short", message))
		}
	}
	return result, nil
}

// dependabotFile reads the configuration under either name Dependabot
// reads; both at once is a stated limit, because which one it obeys is not
// the reader's to decide.
func dependabotFile(in model.Input) (string, []byte, bool, error) {
	const yml, yaml = ".github/dependabot.yml", ".github/dependabot.yaml"
	data, found, err := read(in, yml)
	if err != nil {
		return yml, nil, false, err
	}
	other, present, err := read(in, yaml)
	switch {
	case err != nil:
		return yaml, nil, false, err
	case found && present:
		return yaml, nil, false, outside(yaml, 1, "both "+yml+" and "+yaml+", of which Dependabot reads one; keep one")
	case present:
		return yaml, other, true, nil
	}
	return yml, data, found, nil
}

func cooldownRepairs(update dependabotUpdate) []string {
	var repairs []string
	if update.days[cooldownDefault] < cooldownDays {
		repairs = append(repairs, cooldownFloorRepair(cooldownDefault, cooldownDays))
	}
	if update.ecosystem == dependabotGo {
		for _, key := range []string{cooldownMinor, cooldownPatch} {
			if days, present := update.days[key]; present && days < cooldownDays {
				repairs = append(repairs, cooldownFloorRepair(key, cooldownDays))
			}
		}
		if effectiveCooldown(update, cooldownMajor) < cooldownMajors {
			repairs = append(repairs, cooldownFloorRepair(cooldownMajor, cooldownMajors))
		}
	}
	if len(update.exclude) != 0 {
		repairs = append(repairs, "no exclude list")
	}
	if len(update.include) != 0 && !slices.Contains(update.include, "*") {
		repairs = append(repairs, "include ['*'] or no include list")
	}
	return repairs
}

func effectiveCooldown(update dependabotUpdate, key string) int {
	if value, present := update.days[key]; present {
		return value
	}
	return update.days[cooldownDefault]
}

func cooldownFloorRepair(key string, days int) string {
	return fmt.Sprintf("%s %d or more", key, days)
}

func cooldownMessage(file, ecosystem string, repairs []string) string {
	last := len(repairs) - 1
	text := repairs[last]
	if last > 0 {
		text = strings.Join(repairs[:last], ", ") + " and " + text
	}
	return "set the cooldown of the " + ecosystem + " update in " + file + " to " + text
}
