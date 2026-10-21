package policyfiles

import (
	"slices"

	"gopkg.in/yaml.v3"
)

const (
	buildReaderJobs   = "jobs"
	buildReaderRun    = "run"
	actionsReaderUses = "uses"
)

func readActionsJobs(path string, node *yaml.Node) ([]actionsJob, error) {
	if err := buildReaderMapping(path, node, "Actions jobs"); err != nil {
		return nil, err
	}
	items := entries(node)
	jobs := make([]actionsJob, 0, len(items))
	for _, item := range items {
		job := actionsJob{id: item.key, line: item.line}
		if child(item.value, actionsReaderUses) == nil {
			if err := buildReaderMapping(path, item.value, "Actions job "+item.key, "steps"); err != nil {
				return nil, err
			}
			var err error
			job.steps, err = readActionsSteps(path, child(item.value, "steps"))
			if err != nil {
				return nil, err
			}
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func readActionsSteps(path string, node *yaml.Node) ([]actionsStep, error) {
	if node == nil {
		return nil, nil
	}
	if node.Kind != yaml.SequenceNode {
		return nil, outside(path, node.Line, "Actions steps that are not a sequence")
	}
	steps := make([]actionsStep, 0, len(node.Content))
	for _, item := range node.Content {
		step, err := readActionsStep(path, item)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, nil
}

func readActionsStep(path string, node *yaml.Node) (actionsStep, error) {
	if err := buildReaderMapping(path, node, "Actions step", actionsReaderUses, buildReaderRun, "env"); err != nil {
		return actionsStep{}, err
	}
	step := actionsStep{line: node.Line}
	var err error
	step.uses, err = buildReaderScalar(path, child(node, actionsReaderUses), "Actions step uses")
	if err != nil {
		return actionsStep{}, err
	}
	step.run, err = buildReaderScalar(path, child(node, buildReaderRun), "Actions step run")
	if err != nil {
		return actionsStep{}, err
	}
	step.env, err = readActionsEnvironment(path, child(node, "env"))
	if err != nil {
		return actionsStep{}, err
	}
	return step, nil
}

func readActionsEnvironment(path string, node *yaml.Node) (map[string]string, error) {
	if err := buildReaderMapping(path, node, "Actions step env"); err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, item := range entries(node) {
		value, err := buildReaderScalar(path, resolve(item.value), "Actions step env."+item.key)
		if err != nil {
			return nil, err
		}
		result[item.key] = value
	}
	return result, nil
}

// buildReaderMapping rejects ambiguity only in the fields a reader uses,
// because unrelated metadata cannot change the evidence being retained.
func buildReaderMapping(path string, node *yaml.Node, construct string, keys ...string) error {
	node = resolve(node)
	if node == nil {
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return outside(path, node.Line, construct+" that is not a mapping")
	}
	seen := make(map[string]bool)
	for _, item := range entries(node) {
		if len(keys) != 0 && !slices.Contains(keys, item.key) {
			continue
		}
		if seen[item.key] {
			return outside(path, item.line, "duplicate "+construct+" key "+item.key)
		}
		seen[item.key] = true
	}
	return nil
}

func buildReaderScalar(path string, node *yaml.Node, construct string) (string, error) {
	if node == nil {
		return "", nil
	}
	if node.Kind != yaml.ScalarNode {
		return "", outside(path, node.Line, construct+" that is not scalar text")
	}
	return node.Value, nil
}
