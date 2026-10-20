package cicheck_test

import (
	"maps"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

// hookCommand is one command of a lefthook hook.
type hookCommand struct {
	Run      string `yaml:"run"`
	UseStdin bool   `yaml:"use_stdin"`
}

// lefthookConfig is the part of lefthook.yml the rungs are made of.
type lefthookConfig struct {
	MinVersion string `yaml:"min_version"`
	PreCommit  struct {
		Commands map[string]hookCommand `yaml:"commands"`
	} `yaml:"pre-commit"`
	CommitMsg struct {
		Commands map[string]hookCommand `yaml:"commands"`
	} `yaml:"commit-msg"`
	PrePush struct {
		Commands map[string]hookCommand `yaml:"commands"`
	} `yaml:"pre-push"`
}

// TestKitRungsCallTheBinaryOnly holds the kit's own lefthook.yml to one
// call of the binary on PATH per rung (sections 7 and 10.5): the released
// binary at the pin, never a build of the tree a commit changes, nothing
// heavier in a hook, commit-msg with the message file and pre-push with the
// remote, the URL and the ref lines on stdin.
func TestKitRungsCallTheBinaryOnly(t *testing.T) {
	t.Parallel()
	var config lefthookConfig
	if err := yaml.Unmarshal(kitBytes(t, "lefthook.yml"), &config); err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]hookCommand{
		"pre-commit": {"golaw": {Run: "golaw hook pre-commit"}},
		"commit-msg": {"golaw": {Run: "golaw hook commit-msg '{1}'"}},
		"pre-push":   {"golaw": {Run: "golaw hook pre-push '{1}' '{2}'", UseStdin: true}},
	}
	got := map[string]map[string]hookCommand{
		"pre-commit": config.PreCommit.Commands, "commit-msg": config.CommitMsg.Commands, "pre-push": config.PrePush.Commands,
	}
	for _, rung := range slices.Sorted(maps.Keys(want)) {
		if !maps.Equal(got[rung], want[rung]) {
			t.Errorf("%s runs %+v, want %+v", rung, got[rung], want[rung])
		}
	}
	if config.MinVersion != "2.1.0" {
		t.Errorf("min_version = %q", config.MinVersion)
	}
}
