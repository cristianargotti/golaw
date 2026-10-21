package policyfiles

import (
	"errors"
	"path"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	actionsReaderSteps = "steps"
	demandFalse        = "false"
	demandDirectoryKey = "working-directory"
)

// demandSnapshot lists one Actions file, so readActions reads that file
// alone and a limit of another file never hides its evidence.
type demandSnapshot struct {
	model.Snapshot

	file model.File
}

// Files answers the one file the snapshot was narrowed to.
func (s demandSnapshot) Files() []model.File { return []model.File{s.file} }

// demandTargets answers the targets a scheduled or manually dispatched job
// runs, file by file, beside the first limit met, because evidence another
// file proves completes the coverage a limited file cannot.
func demandTargets(in model.Input) ([]string, error) {
	files := model.Select(in.Snapshot, func(name string) bool {
		return path.Dir(name) == ".github/workflows" && (path.Ext(name) == ".yml" || path.Ext(name) == ".yaml")
	})
	slices.SortFunc(files, func(a, b model.File) int { return strings.Compare(a.Path, b.Path) })
	var evidence ciEvidence
	for _, file := range files {
		targets, err := demandFile(in, file)
		evidence.add(targets, err)
	}
	return evidence.targets, evidence.limit
}

// demandFile reads the events first, so a file no on-demand event triggers
// supplies nothing and limits nothing, then the jobs, one by one when the
// file holds a construct outside the reader's subset.
func demandFile(in model.Input, file model.File) ([]string, error) {
	data, found, err := read(in, file.Path)
	if err != nil || !found {
		return nil, err
	}
	root, err := decodeYAML(file.Path, data)
	if err != nil {
		return nil, err
	}
	if mappingErr := buildReaderMapping(file.Path, root, "Actions events", "on", buildReaderJobs); mappingErr != nil {
		return nil, mappingErr
	}
	triggers, err := readActionsTriggers(file.Path, child(root, "on"))
	if err != nil {
		return nil, err
	}
	if !slices.ContainsFunc(triggers, func(event string) bool { return slices.Contains(demandEvents(), event) }) {
		return nil, nil
	}
	in.Snapshot = demandSnapshot{Snapshot: in.Snapshot, file: file}
	actions, err := readActions(in)
	if errors.Is(err, model.ErrOutsideSubset) {
		return demandPartialJobs(file.Path, root)
	}
	if err != nil {
		return nil, err
	}
	return demandJobs(file.Path, root, actions[0].jobs)
}

func demandJobs(file string, root *yaml.Node, jobs []actionsJob) ([]string, error) {
	var evidence ciEvidence
	for _, job := range jobs {
		node := child(root, buildReaderJobs, job.id)
		targets, err := demandJob(file, root, node, job)
		evidence.add(targets, err)
	}
	return evidence.targets, evidence.limit
}

// A limit in one job cannot erase the independently read evidence of another.
func demandPartialJobs(file string, root *yaml.Node) ([]string, error) {
	node := child(root, buildReaderJobs)
	if err := buildReaderMapping(file, node, "Actions jobs"); err != nil {
		return nil, err
	}
	var evidence ciEvidence
	for _, item := range entries(node) {
		section := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: item.key, Line: item.line}, item.value,
		}}
		jobs, err := readActionsJobs(file, section)
		if err != nil {
			evidence.add(nil, err)
			continue
		}
		targets, err := demandJobs(file, root, jobs)
		evidence.add(targets, err)
	}
	return evidence.targets, evidence.limit
}

// demandJob reads the controls of a step only when its text could invoke
// make, so a condition or a runner the evidence does not depend on never
// becomes a limit; a reusable job supplies no target and may hide one.
func demandJob(file string, root, node *yaml.Node, job actionsJob) ([]string, error) {
	if child(node, actionsReaderUses) != nil && !demandDisabled(node) {
		return nil, outside(file, job.line, "reusable Actions job "+job.id)
	}
	var evidence ciEvidence
	steps := child(node, actionsReaderSteps)
	for i, step := range job.steps {
		stepNode := resolve(steps.Content[i])
		if !demandRelevant(step.run) || demandDisabled(node) || demandDisabled(stepNode) {
			continue
		}
		err := demandControls(file, root, node, stepNode)
		if err != nil {
			evidence.add(nil, err)
			continue
		}
		run := child(stepNode, buildReaderRun)
		step.line = run.Line
		if run.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
			step.line++
		}
		targets, err := demandRun(file, step)
		evidence.add(targets, err)
	}
	return evidence.targets, evidence.limit
}

// demandRun reads each plain line of a run text: a standalone literal make
// call is evidence, a line the shell could redirect, suppress or move is a
// limit of the whole step, and any other line is no evidence.
func demandRun(file string, step actionsStep) ([]string, error) {
	var targets []string
	for offset, text := range strings.Split(step.run, "\n") {
		line := strings.TrimSpace(text)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		words := strings.Fields(line)
		if demandShellControl(line, words[0]) || demandMakeEnvironment(line) {
			return nil, outside(file, step.line+offset, "Actions run: "+line)
		}
		if words[0] != ciMakeCommand {
			continue
		}
		found, ok := recipeTargets(line)
		if !ok {
			return nil, outside(file, step.line+offset, "Actions run: "+line)
		}
		targets = append(targets, found...)
	}
	return targets, nil
}

// demandShellControl recognizes the shell syntax that can change whether
// or where a later make line runs or fails, without interpreting it.
func demandShellControl(line, command string) bool {
	if strings.ContainsAny(line, ";&|<>`\\") || strings.Count(line, "'")%2 != 0 || strings.Count(line, `"`)%2 != 0 {
		return true
	}
	return slices.Contains([]string{
		"if", "then", "else", "elif", "fi", "for", "while", "until", "do", "done", "case", "esac",
		"select", "function", "{", "}", "(", ")", "!", "set", "cd", "pushd", "popd", "trap", "exec", "exit", "return", "eval", "source", ".",
	}, command) || strings.HasPrefix(command, "-") || strings.HasPrefix(command, "@") || strings.Contains(command, "$(")
}
