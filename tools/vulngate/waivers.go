package main

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// waiver is one entry of vulnerability_waivers in golaw.yml (A6).
type waiver struct {
	ID     string `yaml:"id"`
	Until  string `yaml:"until"`
	Record string `yaml:"record"`
}

// policy is the part of golaw.yml the tool reads; every other key is the
// policy package's and ignored here.
type policy struct {
	Waivers []waiver `yaml:"vulnerability_waivers"`
}

// entry is a validated waiver.
type entry struct {
	until  time.Time
	record string
}

// loadWaivers reads and validates the waivers of the policy file.
func loadWaivers(path string) (map[string]entry, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the policy path is a flag of this tool
	if err != nil {
		return nil, fmt.Errorf("read waivers: %w", err)
	}
	var p policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("decode waivers: %w", err)
	}
	out := make(map[string]entry, len(p.Waivers))
	for _, w := range p.Waivers {
		if w.ID == "" || w.Record == "" {
			return nil, fmt.Errorf("%w: every waiver needs an id and a record", ErrWaiver)
		}
		until, err := time.Parse(time.DateOnly, w.Until)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: until: %w", ErrWaiver, w.ID, err)
		}
		out[w.ID] = entry{until: until, record: w.Record}
	}
	return out, nil
}
