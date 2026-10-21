package assets

import "gopkg.in/yaml.v3"

// RunStep keeps literal commands and their context available for ordered replay.
type RunStep struct {
	Job  string
	Name string
	Run  string
	Env  map[string]string
}

// RunSteps leaves out the steps that use an action, because a replay brings
// its own tools, and leaves execution eligibility to the caller's context.
func RunSteps(data []byte) ([]RunStep, error) {
	node, err := ciDocument(data)
	if err != nil {
		return nil, err
	}
	root, err := ciMapping(node, ciRoot)
	if err != nil {
		return nil, err
	}
	jobs, ok := root["jobs"]
	if !ok {
		return nil, ciError(node, ciRoot, "missing jobs mapping")
	}
	return ciJobs(jobs)
}

func ciJobs(node *yaml.Node) ([]RunStep, error) {
	if _, err := ciMapping(node, "jobs"); err != nil {
		return nil, err
	}
	steps := make([]RunStep, 0)
	for i := 0; i < len(node.Content); i += 2 {
		job, err := ciJob(node.Content[i].Value, node.Content[i+1])
		if err != nil {
			return nil, err
		}
		steps = append(steps, job...)
	}
	return steps, nil
}

func ciJob(id string, node *yaml.Node) ([]RunStep, error) {
	location := "jobs." + id
	job, err := ciMapping(node, location)
	if err != nil {
		return nil, err
	}
	env, err := ciEnvironment(job["env"], location+".env")
	if err != nil {
		return nil, err
	}
	sequence, ok := job["steps"]
	location += ".steps"
	if !ok {
		return nil, ciError(node, location, "missing sequence")
	}
	if sequence.Kind == yaml.AliasNode {
		return nil, ciError(sequence, location, ciAlias)
	}
	if sequence.Kind != yaml.SequenceNode {
		return nil, ciError(sequence, location, "expected sequence")
	}
	steps := make([]RunStep, 0, len(sequence.Content))
	for _, item := range sequence.Content {
		step, include, err := ciStep(id, item, env)
		if err != nil {
			return nil, err
		}
		if include {
			steps = append(steps, step)
		}
	}
	return steps, nil
}

func ciStep(job string, node *yaml.Node, inherited map[string]string) (RunStep, bool, error) {
	location := "jobs." + job + ".steps"
	fields, err := ciMapping(node, location)
	if err != nil {
		return RunStep{}, false, err
	}
	_, run := fields["run"]
	uses, action := fields["uses"]
	if run == action {
		return RunStep{}, false, ciError(node, location, "expected exactly one run or uses")
	}
	if action {
		_, err = ciCommand(uses, location+".uses")
		return RunStep{}, false, err
	}
	step, err := ciRunStep(job, location, fields, inherited)
	return step, err == nil, err
}

func ciRunStep(job, location string, fields map[string]*yaml.Node, inherited map[string]string) (RunStep, error) {
	command, err := ciCommand(fields["run"], location+".run")
	if err != nil {
		return RunStep{}, err
	}
	name := ""
	if node, ok := fields["name"]; ok {
		name, err = ciScalar(node, location+".name")
		if err != nil {
			return RunStep{}, err
		}
	}
	local, err := ciEnvironment(fields["env"], location+".env")
	if err != nil {
		return RunStep{}, err
	}
	return RunStep{Job: job, Name: name, Run: command, Env: ciMergedEnvironment(inherited, local)}, nil
}

func ciCommand(node *yaml.Node, location string) (string, error) {
	value, err := ciScalar(node, location)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", ciError(node, location, "expected nonempty command")
	}
	return value, nil
}
