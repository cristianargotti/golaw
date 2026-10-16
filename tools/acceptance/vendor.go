package main

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	minimumMajor   = 2
	minimumMinor   = 1
	minimumPatch   = 295
	versionParts   = 3
	categoryVendor = "vendor"
	caseVersion    = "version"
	vendorRowCount = 3
)

func (s *suite) vendorRows(ctx context.Context, required bool) ([]row, string) {
	binary, err := s.lookup(agentClaude)
	if err != nil {
		return []row{
			{categoryVendor, "plugin-validate", statusSkip, "Claude executable absent", required},
			pluginTestRow(),
		}, "absent"
	}
	got := s.execute(ctx, command{
		program: binary, args: []string{"--version"}, dir: s.root, env: s.env,
	})
	version := strings.TrimSpace(got.stdout)
	if err := success(got); err != nil {
		return []row{measured(categoryVendor, caseVersion,
			fmt.Errorf("read agent version: %w", err))}, version
	}
	if err := versionFloor(version); err != nil {
		return []row{measured(categoryVendor, caseVersion, err)}, version
	}
	rows := make([]row, 0, vendorRowCount)
	rows = append(rows, row{categoryVendor, caseVersion, statusPass, version, true})
	plugin := filepath.Dir(filepath.Dir(s.hookFile))
	got = s.execute(ctx, command{
		program: binary, args: []string{"plugin", "validate", "--strict", plugin}, dir: s.root, env: s.env,
	})
	rows = append(rows, measured(categoryVendor, "plugin-validate", success(got)), pluginTestRow())
	return rows, version
}

// pluginTestRow records why the host's plugin test command never runs: it
// is a runner for hook modules written against the host's testing kit, and
// the plugin ships command hooks and no module, so the command refuses it.
func pluginTestRow() row {
	return row{categoryVendor, "plugin-test", statusSkip, "the host's plugin test runs hook modules; the plugin ships command hooks and no module", false}
}

func versionFloor(text string) error {
	pattern := regexp.MustCompile(`(?:^|\D)(\d+)\.(\d+)\.(\d+)(?:\D|$)`)
	match := pattern.FindStringSubmatch(text)
	if len(match) != versionParts+1 {
		return fmt.Errorf("parse agent version %q: %w", text, ErrAcceptance)
	}
	actual := make([]int, versionParts)
	for i := range versionParts {
		value, err := strconv.Atoi(match[i+1])
		if err != nil {
			return fmt.Errorf("parse agent version number: %w", err)
		}
		actual[i] = value
	}
	if slices.Compare(actual, []int{minimumMajor, minimumMinor, minimumPatch}) < 0 {
		return fmt.Errorf("agent version %q is below 2.1.295: %w", text, ErrAcceptance)
	}
	return nil
}
