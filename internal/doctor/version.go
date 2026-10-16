package doctor

import (
	"fmt"
	"strconv"
	"strings"
)

// VersionAtLeast compares numeric components without adding a module dependency.
func VersionAtLeast(output, floor string) (bool, error) {
	current, err := parseVersion(output)
	if err != nil {
		return false, err
	}
	minimum, err := parseVersion(floor)
	if err != nil {
		return false, err
	}
	for index, value := range current {
		if value != minimum[index] {
			return value > minimum[index], nil
		}
	}
	return true, nil
}

func parseVersion(output string) ([3]int, error) {
	var result [3]int
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return result, fmt.Errorf("empty version: %w", ErrVersion)
	}
	parts := strings.Split(fields[0], ".")
	if len(parts) != len(result) {
		return result, fmt.Errorf("version component count: %w", ErrVersion)
	}
	for index, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return result, fmt.Errorf("version component %q: %w", part, ErrVersion)
		}
		result[index] = value
	}
	return result, nil
}
