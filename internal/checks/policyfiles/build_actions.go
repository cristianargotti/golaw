package policyfiles

import (
	"path"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
)

// actionsFile is one Actions file as the build and lint checks read it.
type actionsFile struct {
	path     string       // Repository-relative path of the file.
	triggers []string     // Event names under on, sorted and unique.
	jobs     []actionsJob // Jobs in file order.
}

// actionsJob keeps the job line so a limit or a finding names the job.
type actionsJob struct {
	id    string        // Job id, the key under jobs.
	steps []actionsStep // Local steps in order, none for a reusable job.
	line  int           // One-based line of the job key.
}

// actionsStep keeps the step text unexecuted, because a check reads facts.
type actionsStep struct {
	uses, run string            // One of the two is set.
	env       map[string]string // The env of the step.
	line      int               // One-based line of the step.
}

// readActions reads only direct YAML children of .github/workflows/ through
// the snapshot, so index and candidate content have the same semantics. Its
// subset covers literal event names, ordered local jobs and steps, scalar
// uses/run text and step environment mappings, with aliases and merges.
// Reusable jobs retain their identity without supplying local evidence;
// neither remote code nor shell text is expanded or executed.
func readActions(in model.Input) ([]actionsFile, error) {
	files := model.Select(in.Snapshot, func(name string) bool {
		return path.Dir(name) == ".github/workflows" && (path.Ext(name) == ".yml" || path.Ext(name) == ".yaml")
	})
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	slices.Sort(paths)
	var result []actionsFile
	for _, name := range paths {
		data, found, err := read(in, name)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		file, err := readActionsFile(name, data)
		if err != nil {
			return nil, err
		}
		result = append(result, file)
	}
	return result, nil
}

func readActionsFile(file string, data []byte) (actionsFile, error) {
	root, err := decodeYAML(file, data)
	if err != nil {
		return actionsFile{}, err
	}
	if mappingErr := buildReaderMapping(file, root, "Actions file", "on", buildReaderJobs); mappingErr != nil {
		return actionsFile{}, mappingErr
	}
	triggers, err := readActionsTriggers(file, child(root, "on"))
	if err != nil {
		return actionsFile{}, err
	}
	jobs, err := readActionsJobs(file, child(root, buildReaderJobs))
	if err != nil {
		return actionsFile{}, err
	}
	return actionsFile{path: file, triggers: triggers, jobs: jobs}, nil
}

func readActionsTriggers(file string, node *yaml.Node) ([]string, error) {
	if node == nil {
		return nil, nil
	}
	var nodes []*yaml.Node
	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			nodes = append(nodes, node.Content[i])
		}
	case yaml.SequenceNode:
		nodes = node.Content
	case yaml.DocumentNode, yaml.ScalarNode, yaml.AliasNode:
		nodes = []*yaml.Node{node}
	}
	triggers := make([]string, 0, len(nodes))
	for _, item := range nodes {
		value := resolve(item)
		if value.Kind != yaml.ScalarNode || strings.Contains(value.Value, "${{") {
			return nil, outside(file, item.Line, "an Actions on event name that is not literal scalar text")
		}
		triggers = append(triggers, value.Value)
	}
	slices.Sort(triggers)
	return slices.Compact(triggers), nil
}
