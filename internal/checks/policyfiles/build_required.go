package policyfiles

import (
	"fmt"
	"path"
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
)

const buildMakefile = "Makefile"

// Makefile is policy.makefile: the Makefile and its includes define every
// target the ci target must carry under the capabilities in force. It
// judges existence only where the declared reading is decisive, so an
// expression that could define a missing target is a stated limit.
func Makefile() model.Check {
	return buildCheck{metadata: spec("makefile", ruleLadder, "go"), judge: judgeMakefile}
}

// judgeMakefile reads the root Makefile, then its missing includes, then the
// required targets, because a target an absent include would define cannot
// be judged until the include is restored.
func judgeMakefile(in model.Input) model.Result {
	snapshot := &buildSnapshot{Snapshot: in.Snapshot}
	in.Snapshot = snapshot
	_, found, err := read(in, buildMakefile)
	if err != nil {
		return answer(snapshot.failed, err)
	}
	if !found {
		return makeFileMissing(in)
	}
	m, err := readMakefile(in, buildMakefile)
	if err != nil {
		return answer(snapshot.failed, err)
	}
	findings, err := makeMissingIncludes(m)
	if err != nil {
		return answer(buildMakefile, err)
	}
	if len(findings) != 0 {
		return model.Result{Status: model.Measured(), Findings: findings}
	}
	return makeRequiredTargets(m, in.Policy.Capabilities)
}

// makeFileMissing names an absent Makefile, unless the snapshot holds a
// make file of another name GNU make reads, which golaw does not: that is a
// stated limit naming the file and the rename, never a missing Makefile.
func makeFileMissing(in model.Input) model.Result {
	for _, name := range []string{"GNUmakefile", "makefile"} {
		if model.Any(in.Snapshot, func(path string) bool { return path == name }) {
			return answer(name, outside(name, 1, "a make file named "+name+", which golaw does not read; rename it to Makefile"))
		}
	}
	return model.Result{Status: model.Measured(), Findings: []model.Finding{
		finding(buildMakefile, 0, "make-file-missing", "add Makefile with the required targets"),
	}}
}

// makeMissingIncludes names each unconditional required include the
// snapshot does not hold. An optional or conditional include may never be
// read, and an include a rule or an unresolved expression may remake is a
// stated limit, because make builds it before it reads it.
func makeMissingIncludes(m makefile) ([]model.Finding, error) {
	var findings []model.Finding
	for _, include := range m.missing {
		if include.optional || include.condition != "" {
			continue
		}
		if file, found := makeIncludeRule(m, include.name); found {
			return nil, outside(include.file, include.line,
				fmt.Sprintf("include %s, which a rule of %s remakes", include.name, file))
		}
		if err := makeTargetLimit(m, path.Clean(include.name)); err != nil {
			return nil, err
		}
		message := fmt.Sprintf("restore %s or correct its include in %s", include.name, include.file)
		findings = append(findings, finding(include.file, include.line, "make-include-missing", message))
	}
	return findings, nil
}

// makeIncludeRule answers the file of an explicit rule that remakes the
// include, compared as make compares paths.
func makeIncludeRule(m makefile, name string) (string, bool) {
	for _, rule := range m.rules {
		for _, target := range rule.targets {
			if path.Clean(target) == path.Clean(name) {
				return rule.file, true
			}
		}
	}
	return "", false
}

// makeRequiredTargets compares the explicit definitions with the required
// targets: a limit any absent target meets wins over every finding, because
// a stated limit never carries findings.
func makeRequiredTargets(m makefile, capabilities []string) model.Result {
	var findings []model.Finding
	for _, target := range RequiredTargets(capabilities) {
		if _, found := m.rule(target); found {
			findings = append(findings, makeRecipeOverrides(m, target)...)
			continue
		}
		if err := makeTargetLimit(m, target); err != nil {
			return answer(buildMakefile, err)
		}
		message := fmt.Sprintf("add required target %s to Makefile", target)
		findings = append(findings, finding(buildMakefile, 0, "make-target-missing", message))
	}
	return model.Result{Status: model.Measured(), Findings: findings}
}

// makeRecipeOverrides names every later unconditional rule line that gives
// a required target a second recipe, because make keeps the last one and
// the kit's recipe or the developer's would silently stop running.
func makeRecipeOverrides(m makefile, target string) []model.Finding {
	var findings []model.Finding
	previous := ""
	for _, rule := range m.rules {
		if rule.condition != "" || rule.recipe == nil || !slices.Contains(rule.targets, target) {
			continue
		}
		if previous != "" {
			message := fmt.Sprintf("target %s has recipes in %s and %s; rename one of the targets", target, previous, rule.file)
			findings = append(findings, finding(rule.file, rule.line, "make-recipe-overridden", message))
		}
		previous = rule.file
	}
	return findings
}
