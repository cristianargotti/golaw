package cicheck_test

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/cristianargotti/golaw/internal/assets"
)

var errM3ToolPin = errors.New("invalid rendered tool pin")

type m3DownloadPin struct {
	Repository    string
	Tag           string
	Asset         string
	ChecksumAsset string
	SHA256        string
}

type m3InstalledPins struct {
	Go        map[string]string
	NPM       map[string]string
	Downloads []m3DownloadPin
}

type m3PinLine struct {
	Job        string
	Step, Line int
	Text       string
}

func m3ToolPins(data []byte) (m3InstalledPins, error) {
	pins := m3InstalledPins{Go: make(map[string]string), NPM: make(map[string]string), Downloads: []m3DownloadPin{}}
	steps, err := assets.RunSteps(data)
	if err != nil {
		return pins, fmt.Errorf("extract installed pins: %w", err)
	}
	for _, line := range m3PinLines(steps) {
		if installErr := m3InstallLine(line, pins); installErr != nil {
			return pins, installErr
		}
	}
	pins.Downloads, err = m3DownloadPins(steps)
	return pins, err
}

func m3PinLines(steps []assets.RunStep) []m3PinLine {
	ordinals := make(map[string]int)
	var lines []m3PinLine
	for _, step := range steps {
		ordinals[step.Job]++
		for index, text := range strings.Split(step.Run, "\n") {
			if text = strings.TrimSpace(text); text != "" {
				lines = append(lines, m3PinLine{Job: step.Job, Step: ordinals[step.Job], Line: index + 1, Text: text})
			}
		}
	}
	return lines
}

func (line m3PinLine) problem(reason string) error {
	return fmt.Errorf("%w: job %s run step %d line %d %q: %s", errM3ToolPin, line.Job, line.Step, line.Line, line.Text, reason)
}

func m3InstallLine(line m3PinLine, pins m3InstalledPins) error {
	words := strings.Fields(line.Text)
	switch words[0] {
	case "go":
		if line.Text == "go version" {
			return nil
		}
		return m3GoInstall(line, words, pins.Go)
	case "npm":
		return m3NPMInstall(line, words, pins.NPM)
	case "curl", "printf", "install", "tar":
		return nil
	default:
		if m3Noninstallation(line.Text) {
			return nil
		}
		return line.problem("use a recognized literal CI command")
	}
}

func m3GoInstall(line m3PinLine, words []string, pins map[string]string) error {
	const linker = "-ldflags=-X=github.com/zricethezav/gitleaks/v8/version.Version="
	if (len(words) != 3 && len(words) != 4) || words[1] != "install" {
		return line.problem("use go install with one module at an exact vX.Y.Z pin")
	}
	module, version, found := strings.CutLast(words[len(words)-1], "@")
	if !found || !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`).MatchString(module) ||
		!regexp.MustCompile(`^v\d+\.\d+\.\d+$`).MatchString(version) {
		return line.problem("pin the Go module to an exact vX.Y.Z version")
	}
	if len(words) == 4 && (module != "github.com/zricethezav/gitleaks/v8" || words[2] != linker+version) {
		return line.problem("use only the gitleaks linker flag with its module version")
	}
	return m3RecordToolPin(line, pins, module, version)
}

func m3NPMInstall(line m3PinLine, words []string, pins map[string]string) error {
	if len(words) != 4 || words[1] != "install" || words[2] != "--global" {
		return line.problem("use npm install --global with one package at an exact X.Y.Z pin")
	}
	name, version, found := strings.CutLast(words[3], "@")
	if !found || !regexp.MustCompile(`^(@[a-z0-9._-]+/)?[a-z0-9._-]+$`).MatchString(name) ||
		!regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(version) {
		return line.problem("pin the npm package to an exact X.Y.Z version")
	}
	return m3RecordToolPin(line, pins, name, version)
}

func m3RecordToolPin(line m3PinLine, pins map[string]string, name, version string) error {
	if previous, ok := pins[name]; ok && previous != version {
		return line.problem(fmt.Sprintf("%s conflicts with its previous version %s", name, previous))
	}
	pins[name] = version
	return nil
}

func m3Noninstallation(command string) bool {
	switch command {
	case `echo "GOTOOLCHAIN=auto" >> "$GITHUB_ENV"`,
		`mkdir -p "$RUNNER_TEMP/golaw-bin"`,
		`echo "$RUNNER_TEMP/golaw-bin" >> "$GITHUB_PATH"`,
		"docker version", "git diff --exit-code":
		return true
	default:
		return regexp.MustCompile(`^make [a-z0-9-]+( [a-z0-9-]+)*$`).MatchString(command)
	}
}
