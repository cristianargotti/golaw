package main

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
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

// loadWaivers fails on the first incomplete entry: a waiver counts only
// with its advisory, its expiry and its decision record, so an entry
// missing one must stop the lane, not waive or vanish in silence. The
// waivers come back sorted by advisory id, as the policy compiles them.
func loadWaivers(path string) ([]model.Waiver, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the policy path is a flag of this tool
	if err != nil {
		return nil, fmt.Errorf("read waivers: %w", err)
	}
	var p policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("decode waivers: %w", err)
	}
	out := make([]model.Waiver, 0, len(p.Waivers))
	for _, w := range p.Waivers {
		if w.ID == "" || w.Record == "" {
			return nil, fmt.Errorf("%w: every waiver needs an id and a record", ErrWaiver)
		}
		if _, err := time.Parse(time.DateOnly, w.Until); err != nil {
			return nil, fmt.Errorf("%w: %s: until: %w", ErrWaiver, w.ID, err)
		}
		out = append(out, model.Waiver{ID: w.ID, Until: w.Until, Record: w.Record})
	}
	slices.SortFunc(out, func(a, b model.Waiver) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}
