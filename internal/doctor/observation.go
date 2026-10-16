package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/cristianargotti/golaw/internal/install"
)

// Observation binds agent environment evidence to a repository and
// definition, stamped with when the hook process ran and on which OS, the
// last dispatch evidence doctor reports (section 5.6).
type Observation struct {
	Identity    string           `json:"identity"`
	HookDigest  string           `json:"hook_digest"`
	Environment AgentEnvironment `json:"environment"`
	SeenAt      time.Time        `json:"seen_at"`
	OS          string           `json:"os"`
}

// The states of the last dispatch evidence.
const (
	DispatchObserved   = "observed"
	DispatchStale      = "stale"
	DispatchUnobserved = "unobserved"
)

// Dispatch is the last evidence that the host ran a hook process for the
// repository: when, on which OS, and whether the observation belongs to
// the hook definition installed now.
type Dispatch struct {
	State  string    `json:"state"`
	SeenAt time.Time `json:"seen_at"`
	OS     string    `json:"os"`
}

// ReadDispatch answers the dispatch evidence of an observation file. An
// absent or unreadable file, another repository's or one that no hook
// process wrote is no evidence; one an earlier hook definition left is
// stale, because the definition installed now never ran.
func ReadDispatch(path, identity, hookDigest string) Dispatch {
	data, err := os.ReadFile(path) //nolint:gosec // The command selects the state observation file.
	if err != nil {
		return Dispatch{State: DispatchUnobserved}
	}
	var observation Observation
	if json.Unmarshal(data, &observation) != nil || observation.Identity != identity ||
		observation.Environment.Source != sourceAgent {
		return Dispatch{State: DispatchUnobserved}
	}
	dispatch := Dispatch{State: DispatchObserved, SeenAt: observation.SeenAt, OS: observation.OS}
	if observation.HookDigest != hookDigest {
		dispatch.State = DispatchStale
	}
	return dispatch
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
