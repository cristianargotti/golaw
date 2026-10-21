package cicheck_test

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var errM3ActionPin = errors.New("invalid action pin")

func m3PinDocuments(t *testing.T) map[string][]byte {
	t.Helper()
	sets := m3Capabilities()
	documents := make(map[string][]byte, len(sets))
	for _, capabilities := range sets {
		name := strings.Join(capabilities, ",")
		if name == "" {
			name = "none"
		}
		documents["rendered/"+name+"/ci.yml"] = m3CI(t, capabilities)
	}
	return documents
}

func m3PinPairs(data []byte) (map[string]string, error) {
	root, err := document(data)
	if err != nil {
		return nil, fmt.Errorf("extract action pins: %w", err)
	}
	pin := regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_./-]+@[0-9a-f]{40}$`)
	version := regexp.MustCompile(`^# v\d+\.\d+\.\d+$`)
	pairs := make(map[string]string)
	for _, use := range named(root, "uses") {
		comment := strings.TrimSpace(use.LineComment)
		if !pin.MatchString(use.Value) || !version.MatchString(comment) {
			return nil, fmt.Errorf("%w: %q requires a 40-hex SHA and inline # vX.Y.Z comment", errM3ActionPin, use.Value)
		}
		action, sha, _ := strings.Cut(use.Value, "@")
		parts := strings.Split(action, "/")
		tag := strings.TrimSpace(strings.TrimPrefix(comment, "#"))
		endpoint := "repos/" + strings.Join(parts[:2], "/") + "/commits/" + tag
		if previous, exists := pairs[endpoint]; exists && previous != sha {
			return nil, fmt.Errorf("%w: %s names conflicting SHAs %s and %s", errM3ActionPin, endpoint, previous, sha)
		}
		pairs[endpoint] = sha
	}
	if len(pairs) == 0 {
		return nil, fmt.Errorf("%w: CI must contain action pins", errM3ActionPin)
	}
	return pairs, nil
}

func m3CheckPinPairs(pairs map[string]string, resolve func(string) (string, error)) error {
	for _, endpoint := range slices.Sorted(maps.Keys(pairs)) {
		expected := pairs[endpoint]
		answer, err := resolve(endpoint)
		if err != nil {
			return fmt.Errorf("resolve %s: expected SHA %s: %w", endpoint, expected, err)
		}
		actual := strings.TrimSpace(answer)
		if actual != expected {
			return fmt.Errorf("%w: %s expected SHA %s, received SHA %q", errM3ActionPin, endpoint, expected, actual)
		}
	}
	return nil
}
