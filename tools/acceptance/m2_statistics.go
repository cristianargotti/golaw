package main

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/cristianargotti/golaw/internal/rung"
)

type m2Sample struct {
	Rung         string `json:"rung"`
	Cache        string `json:"cache"`
	Milliseconds int64  `json:"milliseconds"`
	Code         int    `json:"code"`
}

const (
	m2WarmBudget = 10 * time.Second
	m2ColdCache  = "cold"
	m2WarmCache  = "warm"
)

type m2SampleKey struct {
	rung  string
	cache string
}

type m2Distribution struct {
	raw    []int64
	codes  []int
	median int64
	max    int64
}

func summarizeM2(samples []m2Sample) (string, error) {
	groups, err := m2GroupSamples(samples)
	if err != nil {
		return "", err
	}
	var lines []string
	for _, name := range rung.Names() {
		for _, cache := range []string{m2ColdCache, m2WarmCache} {
			group := groups[m2SampleKey{name, cache}]
			if len(group) == 0 {
				continue
			}
			dist := m2DescribeSamples(group)
			lines = append(lines, fmt.Sprintf("%s %s samples_ms=%v codes=%v count=%d median_ms=%d max_ms=%d",
				name, cache, dist.raw, dist.codes, len(group), dist.median, dist.max))
		}
	}
	return strings.Join(lines, "\n"), nil
}

func budgetM2(samples []m2Sample, budget time.Duration) (string, error) {
	groups, err := m2GroupSamples(samples)
	if err != nil {
		return "", err
	}
	var lines, failures []string
	for _, name := range rung.Names() {
		group := groups[m2SampleKey{name, m2WarmCache}]
		if len(group) == 0 {
			if len(groups[m2SampleKey{name, m2ColdCache}]) != 0 {
				failures = append(failures, name+" lacks warm samples")
			}
			continue
		}
		dist, status := m2DescribeSamples(group), "within"
		if dist.median > budget.Milliseconds() {
			status = "over"
			failures = append(failures, name+" exceeds the warm budget")
		}
		lines = append(lines, fmt.Sprintf("%s warm_median_ms=%d budget_ms=%d %s",
			name, dist.median, budget.Milliseconds(), status))
	}
	comparison := strings.Join(lines, "\n")
	if len(failures) != 0 {
		return comparison, fmt.Errorf("M2 warm comparison: %s: %w", strings.Join(failures, "; "), ErrAcceptance)
	}
	return comparison, nil
}

func m2GroupSamples(samples []m2Sample) (map[m2SampleKey][]m2Sample, error) {
	if len(samples) == 0 {
		return nil, fmt.Errorf("M2 measurements are absent: %w", ErrAcceptance)
	}
	groups := make(map[m2SampleKey][]m2Sample)
	for index, sample := range samples {
		if err := m2ValidateSample(sample); err != nil {
			return nil, fmt.Errorf("M2 sample %d: %w", index, err)
		}
		key := m2SampleKey{sample.Rung, sample.Cache}
		groups[key] = append(groups[key], sample)
	}
	return groups, nil
}

func m2ValidateSample(sample m2Sample) error {
	if !slices.Contains(rung.Names(), sample.Rung) {
		return fmt.Errorf("unknown rung %q: %w", sample.Rung, ErrAcceptance)
	}
	if sample.Cache != m2ColdCache && sample.Cache != m2WarmCache {
		return fmt.Errorf("unknown cache %q: %w", sample.Cache, ErrAcceptance)
	}
	if sample.Milliseconds < 0 {
		return fmt.Errorf("negative duration for %s: %w", sample.Rung, ErrAcceptance)
	}
	if sample.Code < 0 || sample.Rung != rung.PrePush && sample.Code != 0 {
		return fmt.Errorf("invalid exit status %d for %s: %w", sample.Code, sample.Rung, ErrAcceptance)
	}
	return nil
}

func m2DescribeSamples(samples []m2Sample) m2Distribution {
	dist := m2Distribution{
		raw:   make([]int64, 0, len(samples)),
		codes: make([]int, 0, len(samples)),
	}
	for _, sample := range samples {
		dist.raw = append(dist.raw, sample.Milliseconds)
		dist.codes = append(dist.codes, sample.Code)
	}
	sorted := slices.Clone(dist.raw)
	slices.Sort(sorted)
	dist.median, dist.max = sorted[len(sorted)/2], sorted[len(sorted)-1]
	return dist
}
