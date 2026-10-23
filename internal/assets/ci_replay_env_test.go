package assets_test

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
)

func TestCIReplayEnvironmentPrecedence(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		state map[string]string
		step  map[string]string
		want  string
	}{
		{"base", map[string]string{}, map[string]string{}, "VALUE=base"},
		{"export", map[string]string{"VALUE": "export"}, map[string]string{}, "VALUE=export"},
		{"step", map[string]string{"VALUE": "export"}, map[string]string{"VALUE": "step"}, "VALUE=step"},
		{"empty step", map[string]string{"VALUE": "export"}, map[string]string{"VALUE": ""}, "VALUE="},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := ciReplayEnvironment([]string{"VALUE=base", "KEEP=yes"}, test.state, test.step)
			if !slices.Contains(got, test.want) || !slices.Contains(got, "KEEP=yes") {
				t.Fatalf("environment=%q", got)
			}
			if count := ciCountPrefixed(got, "VALUE="); count != 1 {
				t.Fatalf("VALUE appears %d times", count)
			}
		})
	}
}

func TestCIReplayReadsOnlyItsDeclaredExportForm(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, content string
		fail          bool
	}{
		{"restore", "GOTOOLCHAIN=auto\n", false},
		{"CRLF", "GOTOOLCHAIN=auto\r\n", false},
		{"empty value", "VALUE=\n", false},
		{"missing separator", "GOTOOLCHAIN\n", true},
		{"multiline form", "VALUE<<END\nvalue\nEND\n", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "exports")
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := ciReadExports(path)
			if (err != nil) != test.fail {
				t.Fatalf("exports=%v, error=%v", got, err)
			}
			if !test.fail && test.name != "empty value" && got["GOTOOLCHAIN"] != "auto" {
				t.Fatalf("exports=%v", got)
			}
		})
	}
}

// ciReplayEnvironment merges the base environment, the persisted state and
// a step's environment in that order, the later winning, one entry per
// name, so an export or a step value replaces what it shadows.
func ciReplayEnvironment(base []string, state, step map[string]string) []string {
	values := make(map[string]string, len(base)+len(state)+len(step))
	var order []string
	set := func(key, value string) {
		name := ciEnvName(key)
		if _, seen := values[name]; !seen {
			order = append(order, key)
		}
		values[name] = value
	}
	for _, entry := range base {
		key, value, _ := strings.Cut(entry, "=")
		set(key, value)
	}
	for _, layer := range []map[string]string{state, step} {
		for _, key := range slices.Sorted(maps.Keys(layer)) {
			set(key, layer[key])
		}
	}
	merged := make([]string, 0, len(order))
	for _, key := range order {
		merged = append(merged, key+"="+values[ciEnvName(key)])
	}
	return merged
}

// ciCountPrefixed counts the entries of an environment that start with a
// prefix: the times one name appears.
func ciCountPrefixed(environment []string, prefix string) int {
	count := 0
	for _, entry := range environment {
		if strings.HasPrefix(entry, prefix) {
			count++
		}
	}
	return count
}

// ciEnvName is the identity of an environment name: as written, except on
// Windows, where names are case insensitive and the PATH a replay seeds
// must replace the Path the process inherited.
func ciEnvName(key string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(key)
	}
	return key
}

// ciReadExports reads the NAME=value lines of an exports file, the one form
// the generated steps write; a line of any other form, the multiline form
// among them, is an error naming the line, because a value the replay
// would guess at proves nothing.
func ciReadExports(file string) (map[string]string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errCIReplay, err)
	}
	exports := make(map[string]string)
	for number, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || key == "" || strings.ContainsAny(key, "<> \t") {
			return nil, fmt.Errorf("%w: %s line %d is not a NAME=value export: %q", errCIReplay, file, number+1, line)
		}
		exports[key] = value
	}
	return exports, nil
}

// ciReadPaths reads the path additions of a GITHUB_PATH file, one per line.
func ciReadPaths(file string) ([]string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errCIReplay, err)
	}
	return ciLines(string(data)), nil
}

// ciResolvedStep resolves the exact enrollment expression of a step's
// environment to the value the pipeline would supply, the empty one
// included, and refuses any other expression a replay cannot resolve.
func ciResolvedStep(step assets.RunStep, enrollment string) (assets.RunStep, error) {
	const expression = "${{"
	if strings.Contains(step.Run, expression) {
		return assets.RunStep{}, fmt.Errorf("%w: job %s runs an unresolved expression: %q", errCIReplay, step.Job, step.Run)
	}
	resolved := step
	resolved.Env = maps.Clone(step.Env)
	for key, value := range resolved.Env {
		if value == ciEnrollmentExpression {
			resolved.Env[key] = enrollment
			continue
		}
		if strings.Contains(value, expression) {
			return assets.RunStep{}, fmt.Errorf("%w: job %s leaves %s=%q unresolved", errCIReplay, step.Job, key, value)
		}
	}
	return resolved, nil
}

// ciIsGoInstall answers whether a step starts with a go install.
func ciIsGoInstall(step assets.RunStep) bool {
	lines := ciStepLines(step)
	return len(lines) != 0 && strings.HasPrefix(lines[0], "go install ")
}

// ciLines is the nonempty lines of a command text or of a make expansion,
// trimmed.
func ciLines(text string) []string {
	var lines []string
	for line := range strings.SplitSeq(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// ciStepLines is the nonempty lines of a run step's command text.
func ciStepLines(step assets.RunStep) []string {
	return ciLines(step.Run)
}

// ciStepLabel names a step by its name, or by its ordinal in the job when
// the template gave it none.
func ciStepLabel(ordinal int, step assets.RunStep) string {
	if step.Name != "" {
		return "step " + step.Name
	}
	return "step " + strconv.Itoa(ordinal)
}

// ciBounded keeps the head of a stream for an error message.
func ciBounded(data []byte) string {
	const limit = 2048
	return strings.TrimSpace(string(data[:min(len(data), limit)]))
}
