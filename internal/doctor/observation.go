package doctor

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cristianargotti/golaw/internal/install"
)

// Observation binds agent environment evidence to a repository and definition.
type Observation struct {
	Identity    string           `json:"identity"`
	HookDigest  string           `json:"hook_digest"`
	Environment AgentEnvironment `json:"environment"`
}

// SaveObservation is called inside the hook process, never by terminal inference.
func SaveObservation(path string, observation Observation) error {
	data, err := json.Marshal(observation)
	if err != nil {
		return fmt.Errorf("encode agent observation: %w", err)
	}
	if err := install.WriteAtomic(path, append(data, '\n')); err != nil {
		return fmt.Errorf("write agent observation: %w", err)
	}
	return nil
}

// LoadObservation rejects evidence belonging to another repository or definition.
func LoadObservation(path, identity, hookDigest string) (AgentEnvironment, error) {
	data, err := os.ReadFile(path) //nolint:gosec // The command selects the state observation file.
	if err != nil {
		return AgentEnvironment{}, fmt.Errorf("read agent observation: %w", err)
	}
	var observation Observation
	if err := json.Unmarshal(data, &observation); err != nil {
		return AgentEnvironment{}, fmt.Errorf("decode agent observation: %w", err)
	}
	if observation.Identity != identity || observation.HookDigest != hookDigest ||
		observation.Environment.Source != sourceAgent {
		return AgentEnvironment{}, fmt.Errorf("observation binding: %w", ErrEnvironment)
	}
	return observation.Environment, nil
}
